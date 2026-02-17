package markitdown

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/saintfish/chardet"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/encoding/unicode"
)

// PlainTextConverter handles plain text, markdown, JSON, and JSONL files.
type PlainTextConverter struct{}

// NewPlainTextConverter creates a new PlainTextConverter.
func NewPlainTextConverter() *PlainTextConverter {
	return &PlainTextConverter{}
}

func (c *PlainTextConverter) Accepts(info StreamInfo) bool {
	switch info.Extension {
	case ".txt", ".text", ".md", ".markdown", ".json", ".jsonl":
		return true
	}
	mime := strings.ToLower(info.MIMEType)
	if strings.HasPrefix(mime, "text/") {
		return true
	}
	if strings.HasPrefix(mime, "application/json") || strings.HasPrefix(mime, "application/markdown") {
		return true
	}
	return false
}

func (c *PlainTextConverter) Convert(reader io.ReadSeeker, info StreamInfo) (*DocumentConverterResult, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}

	var text string

	// If charset is provided, use it
	if info.Charset != "" {
		enc := lookupEncoding(info.Charset)
		if enc != nil {
			decoded, err := enc.NewDecoder().Bytes(data)
			if err == nil {
				text = string(decoded)
			}
		}
	}

	// If no charset or decoding failed, try detection
	if text == "" {
		text = decodeWithDetection(data)
	}

	return &DocumentConverterResult{
		Markdown: text,
	}, nil
}

// decodeWithDetection detects the encoding of data and decodes it to UTF-8.
func decodeWithDetection(data []byte) string {
	// If data is valid UTF-8 without ambiguity, return as-is
	if utf8.Valid(data) && !hasHighBytes(data) {
		return string(data)
	}

	// If data is valid UTF-8 with multi-byte characters, check if it looks correct
	if utf8.Valid(data) {
		s := string(data)
		if !strings.ContainsRune(s, '\uFFFD') {
			return s
		}
	}

	detector := chardet.NewTextDetector()
	results, err := detector.DetectAll(data)
	if err == nil && len(results) > 0 {
		// Try all detected charsets, preferring CJK encodings for data with
		// high-byte sequences (since chardet often misidentifies CJK as Latin)
		type decodedResult struct {
			text       string
			charset    string
			confidence int
		}
		var candidates []decodedResult
		for _, r := range results {
			enc := lookupEncoding(r.Charset)
			if enc != nil {
				decoded, err := enc.NewDecoder().Bytes(data)
				if err == nil {
					candidates = append(candidates, decodedResult{
						text:       string(decoded),
						charset:    r.Charset,
						confidence: r.Confidence,
					})
				}
			}
		}

		// Score each candidate - prefer ones that produce coherent Unicode
		bestScore := -1
		bestText := ""
		for _, c := range candidates {
			score := scoreDecodedText(c.text, c.confidence)
			if score > bestScore {
				bestScore = score
				bestText = c.text
			}
		}
		if bestText != "" {
			return bestText
		}
	}

	// Fallback: treat as UTF-8
	return string(data)
}

// hasHighBytes checks if data contains bytes > 0x7F.
func hasHighBytes(data []byte) bool {
	for _, b := range data {
		if b > 0x7F {
			return true
		}
	}
	return false
}

// commonCJK contains the ~500 most frequently used CJK characters across
// Chinese and Japanese. Used to distinguish correct CJK decoding from
// accidental CJK output caused by encoding misdetection.
const commonCJK = "的一是不了人我在有他这中大来上个国到说们为你对生能地下过子" +
	"那要就出会也好开后还事多么然于心可她自之年时发后作里如果所" +
	"好成等都没把最而又同它种间其信表安正回力长外内动见把想用前" +
	"这没还自后那面天月日学方又去手电话被从经当被意进面头起第各" +
	"名前年齢住所東京大阪三木英子佐藤太郎橋淳古屋北海道田中山本" +
	"高野村松井田川口石原林森小上下左右男女白黒赤青金木水火土目" +
	"耳手足口気入出分切行見聞話読書食飲買売作使合知思言語文字数" +
	"百千万円時計色形声音楽歌画図体力仕事会社員店場所駅道町市区" +
	"県州世界全部本物花鳥魚犬猫空海山島川池雨雪風雲春夏秋冬朝" +
	"昼夜毎先新古長短広近遠明暗強弱若早速多少高安正直同違反対特" +
	"別親友達家族兄弟姉妹夫妻息娘父母祖曜教室病院医者薬局図書館" +
	"中国日本韓台湾港英米法独現在未然因果関係属性質量度温度圧電" +
	"流通信号記録報告定決取消送届届申込受付済完了開始終止変更" +
	"追加削除修正確認検索選択設定保存印刷実行処理結果成功失敗" +
	"和平安全危険注意必要重要" +
	"人民共产党政府国家社主义经济发展改革建设工业农科技术教育文化" +
	"生活水平提高加强保护环境资源利管理制度法律权利义务责任服务"

// scoreDecodedText scores how "good" a decoded text looks.
// Higher scores indicate more coherent text.
func scoreDecodedText(text string, confidence int) int {
	score := confidence

	// Count coherent character categories
	var commonCJKCount, rareCJKCount, latinCount, controlCount, replacementCount int
	for _, r := range text {
		switch {
		case r == '\uFFFD':
			replacementCount++
		case r < 0x20 && r != '\n' && r != '\r' && r != '\t':
			controlCount++
		case r >= 0x3040 && r <= 0x30FF: // Hiragana + Katakana (Japanese-specific)
			commonCJKCount++
		case r >= 0xFF00 && r <= 0xFFEF: // Fullwidth forms
			commonCJKCount++
		case r >= 0x4E00 && r <= 0x9FFF: // CJK Unified Ideographs
			if strings.ContainsRune(commonCJK, r) {
				commonCJKCount++
			} else {
				rareCJKCount++
			}
		case r >= 0x0041 && r <= 0x007A: // Basic Latin letters
			latinCount++
		}
	}

	// Penalize replacement characters and control characters
	score -= replacementCount * 10
	score -= controlCount * 5

	// Strong bonus for common CJK characters (likely correct decoding)
	score += commonCJKCount * 5

	// Smaller bonus for rare CJK characters (may indicate wrong encoding)
	score += rareCJKCount * 1

	// Bonus for Latin characters
	score += latinCount

	return score
}

// lookupEncoding maps charset names to Go encoding implementations.
func lookupEncoding(charset string) encoding.Encoding {
	switch strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(charset, "-", ""), "_", "")) {
	case "utf8", "utf8bom":
		return unicode.UTF8
	case "utf16le":
		return unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
	case "utf16be":
		return unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM)
	case "iso88591", "latin1":
		return charmap.ISO8859_1
	case "iso88592":
		return charmap.ISO8859_2
	case "iso88595":
		return charmap.ISO8859_5
	case "iso88596":
		return charmap.ISO8859_6
	case "iso88597":
		return charmap.ISO8859_7
	case "iso88598":
		return charmap.ISO8859_8
	case "iso88599":
		return charmap.ISO8859_9
	case "iso885915":
		return charmap.ISO8859_15
	case "windows1250", "cp1250":
		return charmap.Windows1250
	case "windows1251", "cp1251":
		return charmap.Windows1251
	case "windows1252", "cp1252":
		return charmap.Windows1252
	case "windows1253", "cp1253":
		return charmap.Windows1253
	case "windows1254", "cp1254":
		return charmap.Windows1254
	case "windows1255", "cp1255":
		return charmap.Windows1255
	case "windows1256", "cp1256":
		return charmap.Windows1256
	case "koi8r":
		return charmap.KOI8R
	case "shiftjis", "shiftjis2004", "sjis", "cp932", "windows31j":
		return japanese.ShiftJIS
	case "eucjp":
		return japanese.EUCJP
	case "iso2022jp":
		return japanese.ISO2022JP
	case "euckr", "cp949":
		return korean.EUCKR
	case "gb2312", "gbk", "cp936", "gb18030":
		return simplifiedchinese.GBK
	case "big5", "cp950":
		return traditionalchinese.Big5
	case "ascii", "usascii":
		// ASCII is a subset of UTF-8
		return unicode.UTF8
	}
	return nil
}
