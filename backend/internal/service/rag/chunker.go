package rag

import (
	"bufio"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	tiktoken "github.com/pkoukk/tiktoken-go"
)

var markdownHeading = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*$`)

const cl100kSHA256 = "223921b76ee99bde995b7ff738513eef100fb51d18c93597a113bcffe865b2a7"

// Source: https://openaipublic.blob.core.windows.net/encodings/cl100k_base.tiktoken
// The checksum above pins the official vocabulary and prevents runtime drift.
//
//go:embed cl100k_base.tiktoken
var cl100kVocabulary []byte

var (
	initializeTokenizer sync.Once
	cl100kEncoding      *tiktoken.Tiktoken
	cl100kEncodingErr   error
)

type ChunkerConfig struct {
	MaxTokens         int
	OverlapTokens     int
	ChunkingVersion   int
	TokenizerEncoding string
}

type SourceDocument struct {
	ContentID      int64
	ContentVersion int
	Title          string
	Text           string
}

type Chunk struct {
	ChunkIndex      int
	ChunkKey        string
	ChunkingVersion int
	Heading         string
	Text            string
	SourceStart     int
	SourceEnd       int
}

type Chunker struct {
	config       ChunkerConfig
	encoding     *tiktoken.Tiktoken
	tokenCounter func(string) int
	initErr      error
}

func NewChunker(config ChunkerConfig) *Chunker {
	if config.TokenizerEncoding == "" {
		config.TokenizerEncoding = "cl100k_base"
	}
	if config.TokenizerEncoding != "cl100k_base" {
		return &Chunker{config: config, initErr: fmt.Errorf("unsupported tokenizer encoding %q", config.TokenizerEncoding)}
	}
	initializeTokenizer.Do(func() {
		tiktoken.SetBpeLoader(embeddedCL100KLoader{})
		cl100kEncoding, cl100kEncodingErr = tiktoken.GetEncoding(config.TokenizerEncoding)
	})
	chunker := &Chunker{config: config, encoding: cl100kEncoding, initErr: cl100kEncodingErr}
	if cl100kEncoding != nil {
		chunker.tokenCounter = func(text string) int {
			return len(cl100kEncoding.Encode(text, nil, nil))
		}
	}
	return chunker
}

type embeddedCL100KLoader struct{}

func (embeddedCL100KLoader) LoadTiktokenBpe(name string) (map[string]int, error) {
	if !strings.HasSuffix(name, "/cl100k_base.tiktoken") {
		return nil, fmt.Errorf("unsupported embedded tokenizer vocabulary %q", name)
	}
	sum := sha256.Sum256(cl100kVocabulary)
	if hex.EncodeToString(sum[:]) != cl100kSHA256 {
		return nil, errors.New("embedded cl100k_base checksum mismatch")
	}
	ranks := make(map[string]int, 100256)
	scanner := bufio.NewScanner(strings.NewReader(string(cl100kVocabulary)))
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) != 2 {
			return nil, errors.New("invalid embedded cl100k_base row")
		}
		token, err := base64.StdEncoding.DecodeString(parts[0])
		if err != nil {
			return nil, fmt.Errorf("decode embedded cl100k_base token: %w", err)
		}
		rank, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("decode embedded cl100k_base rank: %w", err)
		}
		ranks[string(token)] = rank
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read embedded cl100k_base: %w", err)
	}
	return ranks, nil
}

func (c *Chunker) Chunk(doc SourceDocument) ([]Chunk, error) {
	if c.config.MaxTokens <= 0 || c.config.OverlapTokens < 0 || c.config.OverlapTokens >= c.config.MaxTokens {
		return nil, errors.New("invalid chunker token limits")
	}
	if c.config.ChunkingVersion <= 0 {
		return nil, errors.New("chunking version must be positive")
	}
	if c.initErr != nil {
		return nil, fmt.Errorf("initialize cl100k_base encoding: %w", c.initErr)
	}
	if c.encoding == nil {
		return nil, errors.New("initialize cl100k_base encoding: nil encoding")
	}
	if strings.TrimSpace(doc.Text) == "" {
		return []Chunk{}, nil
	}

	docRunes := []rune(doc.Text)
	units := parseUnits(doc.Text, docRunes)
	for i := range units {
		units[i].prefix = units[i].heading
		if units[i].prefix == "" {
			units[i].prefix = strings.TrimSpace(doc.Title)
		}
	}
	units = c.mergeUnits(docRunes, units)
	var chunks []Chunk
	for _, unit := range units {
		parts, err := c.splitUnit(docRunes, unit)
		if err != nil {
			return nil, err
		}
		for _, part := range parts {
			for part.start < part.end && unicode.IsSpace(docRunes[part.start]) {
				part.start++
			}
			for part.end > part.start && unicode.IsSpace(docRunes[part.end-1]) {
				part.end--
			}
			partText := string(docRunes[part.start:part.end])
			if partText == "" {
				continue
			}
			storedText := partText
			if unit.prefix != "" {
				storedText = unit.prefix + "\n" + partText
			}
			textHash := sha256.Sum256([]byte(storedText))
			identity := fmt.Sprintf("%d/%d/%d/%d/%d/%s", doc.ContentID, doc.ContentVersion, c.config.ChunkingVersion, part.start, part.end, hex.EncodeToString(textHash[:]))
			key := sha256.Sum256([]byte(identity))
			chunks = append(chunks, Chunk{
				ChunkIndex:      len(chunks),
				ChunkKey:        hex.EncodeToString(key[:]),
				ChunkingVersion: c.config.ChunkingVersion,
				Heading:         unit.heading,
				Text:            storedText,
				SourceStart:     part.start,
				SourceEnd:       part.end,
			})
		}
	}
	return chunks, nil
}

func (c *Chunker) mergeUnits(runes []rune, units []sourceUnit) []sourceUnit {
	if len(units) < 2 {
		return units
	}
	merged := make([]sourceUnit, 0, len(units))
	current := units[0]
	for _, next := range units[1:] {
		if next.prefix == current.prefix && c.tokenCount(current.prefix, string(runes[current.start:next.end])) <= c.config.MaxTokens {
			current.end = next.end
			continue
		}
		merged = append(merged, current)
		current = next
	}
	return append(merged, current)
}

type sourceUnit struct {
	heading    string
	prefix     string
	start, end int
}

func parseUnits(text string, runes []rune) []sourceUnit {
	lines := strings.SplitAfter(text, "\n")
	position := 0
	headingChain := make([]string, 0, 6)
	unitStart := 0
	currentHeading := ""
	var units []sourceUnit
	flush := func(end int) {
		if end > unitStart && strings.TrimSpace(string(runes[unitStart:end])) != "" {
			units = append(units, sourceUnit{heading: currentHeading, start: unitStart, end: end})
		}
		unitStart = end
	}
	for _, line := range lines {
		lineRunes := utf8.RuneCountInString(line)
		trimmed := strings.TrimSpace(line)
		if match := markdownHeading.FindStringSubmatch(trimmed); match != nil {
			flush(position)
			level := len(match[1])
			headingChain = headingChain[:min(level-1, len(headingChain))]
			headingChain = append(headingChain, match[2])
			currentHeading = strings.Join(headingChain, " > ")
			unitStart = position + lineRunes
		} else if trimmed == "" {
			flush(position + lineRunes)
		}
		position += lineRunes
	}
	flush(len(runes))
	if len(units) == 0 {
		return []sourceUnit{{start: 0, end: len(runes)}}
	}
	return units
}

type sourcePart struct{ start, end int }

func (c *Chunker) splitUnit(runes []rune, unit sourceUnit) ([]sourcePart, error) {
	parts := make([]sourcePart, 0)
	for start := unit.start; start < unit.end; {
		best := start
		high := start + 1
		for {
			if c.tokenCount(unit.prefix, string(runes[start:high])) > c.config.MaxTokens {
				break
			}
			best = high
			if high == unit.end {
				break
			}
			width := high - start
			high = start + min(width*2, unit.end-start)
		}
		// Exponential expansion keeps each chunk local even when a tiny token
		// budget is applied to a multi-megabyte, highly compressible document.
		// The last passing window is used directly: refining toward the failing
		// bound would repeatedly retokenize nearly identical text and recreate
		// the pathological cost this bounded-window algorithm avoids.
		if best == start {
			return nil, errors.New("chunk prefix leaves no token budget for source text")
		}
		if best < unit.end {
			for boundary := best - 1; boundary > start; boundary-- {
				if strings.ContainsRune("。！？.!?\n", runes[boundary]) {
					best = boundary + 1
					break
				}
			}
		}
		parts = append(parts, sourcePart{start: start, end: best})
		if best >= unit.end {
			break
		}
		next := best
		if c.config.OverlapTokens > 0 && best-1 > start {
			next = best - 1
			step := 2
			failed := start
			for next > start+1 {
				candidate := max(start+1, best-step)
				if c.tokenCount("", string(runes[candidate:best])) > c.config.OverlapTokens {
					failed = candidate
					break
				}
				next = candidate
				if candidate == start+1 {
					break
				}
				step *= 2
			}
			low, high := failed+1, next
			if failed == start {
				low = start + 1
			}
			for low < high {
				candidate := low + (high-low)/2
				if c.tokenCount("", string(runes[candidate:best])) <= c.config.OverlapTokens {
					high = candidate
				} else {
					low = candidate + 1
				}
			}
			next = low
		}
		if next <= start {
			next = best
		}
		start = next
	}
	return parts, nil
}

func (c *Chunker) tokenCount(heading, text string) int {
	input := text
	if heading != "" {
		input = heading + "\n" + text
	}
	return c.tokenCounter(input)
}
