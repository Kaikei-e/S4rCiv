package legislative

// e-Gov 法令 (egov-law) interpretation-plane entities and the 法令標準XML parser.
// Like the kokkai entities, every value is derived deterministically from an
// observation snapshot so projection stays reproject-safe.
//
// The eId derivation here is a SHARED CONTRACT with the Rust differ: the same
// element id must be produced byte-for-byte on both sides so a reported change
// (interpretation.change node refs) joins to its node row. See ADR-000005 and the
// migration comment on interpretation.law_node.

import (
	"encoding/xml"
	"strconv"
	"strings"
	"time"
)

// Node types stored in interpretation.law_node.node_type.
const (
	NodeArticle   = "article"
	NodeParagraph = "paragraph"
	NodeItem      = "item"
)

// Law is one 法令 (= one observation stream, keyed by the stable e-Gov 法令ID).
// It observes the law's 現行 (in-force consolidated) 法令標準XML.
type Law struct {
	LawID                    string
	StreamID                 string
	LawNum                   string // 法令番号
	LawType                  string // e-Gov enum: Act / CabinetOrder / ...
	Title                    string
	TitleKana                string
	Category                 string
	PromulgationDate         string // YYYY-MM-DD
	CurrentRevisionID        string // observed law_revision_id ({law_id}_{施行日}_{改正法令ID})
	AmendmentEnforcementDate string // YYYY-MM-DD, 当該版の施行日
	CurrentRevisionStatus    string
	RepealStatus             string
	RepealDate               string // YYYY-MM-DD
	Permalink                string // e-Gov reference URL (attribution)
}

// LawNode is one normative node (条/項/号) in the current tree.
type LawNode struct {
	EID          string
	ParentEID    string
	NodeType     string // article | paragraph | item
	Num          string // 条/項/号 number token ('9', '9_2' for 第9条の2)
	Caption      string // 見出し (ArticleCaption); articles only
	ChapterNum   string // enclosing Chapter Num (container metadata)
	SectionNum   string // enclosing Section Num (container metadata)
	IsSuppl      bool   // under a SupplProvision (附則) vs MainProvision (本則)
	SentenceText string // concatenated normative <Sentence> text owned by the node
	Ordinal      int    // 0-based document-order counter over all materialized nodes
}

// LawContent is the full normalized parse of one law snapshot.
type LawContent struct {
	Law               Law
	Nodes             []LawNode
	SourcePublishedAt *time.Time
}

// LawStreamID is the deterministic stream identity for an e-Gov 法令ID.
func LawStreamID(lawID string) string { return "egov-law:" + lawID }

// ── 法令標準XML decoding shapes ──────────────────────────────────────────────
// Only the structural elements the eId contract materializes are typed; unknown
// elements are ignored by encoding/xml.

type xmlLaw struct {
	XMLName xml.Name `xml:"Law"`
	LawNum  string   `xml:"LawNum"`
	Body    xmlBody  `xml:"LawBody"`
}

type xmlBody struct {
	LawTitle       xmlLawTitle    `xml:"LawTitle"`
	MainProvision  xmlProvision   `xml:"MainProvision"`
	SupplProvision []xmlProvision `xml:"SupplProvision"`
}

type xmlLawTitle struct {
	Kana  string `xml:"Kana,attr"`
	Value string `xml:",chardata"`
}

// xmlProvision is a MainProvision or SupplProvision. Articles may be nested under
// container elements (Part/Chapter/Section/Subsection/Division) or appear directly.
type xmlProvision struct {
	Parts    []xmlContainer `xml:"Part"`
	Chapters []xmlContainer `xml:"Chapter"`
	Articles []xmlArticle   `xml:"Article"`
}

type xmlContainer struct {
	Num         string         `xml:"Num,attr"`
	Chapters    []xmlContainer `xml:"Chapter"`
	Sections    []xmlContainer `xml:"Section"`
	Subsections []xmlContainer `xml:"Subsection"`
	Divisions   []xmlContainer `xml:"Division"`
	Articles    []xmlArticle   `xml:"Article"`
}

type xmlArticle struct {
	Num        string         `xml:"Num,attr"`
	Caption    string         `xml:"ArticleCaption"`
	Title      string         `xml:"ArticleTitle"`
	Paragraphs []xmlParagraph `xml:"Paragraph"`
}

type xmlParagraph struct {
	Num               string         `xml:"Num,attr"`
	ParagraphSentence xmlSentenceSet `xml:"ParagraphSentence"`
	Items             []xmlItem      `xml:"Item"`
}

type xmlItem struct {
	Num          string         `xml:"Num,attr"`
	ItemSentence xmlSentenceSet `xml:"ItemSentence"`
}

type xmlSentenceSet struct {
	Sentences []string `xml:"Sentence"`
}

// containerKind tags which Num column an enclosing container fills.
type containerKind int

const (
	containerOther containerKind = iota
	containerChapter
	containerSection
)

// scope carries the enclosing container metadata down the tree during a walk.
type scope struct {
	chapterNum string
	sectionNum string
}

// builder accumulates nodes with a single document-order ordinal counter.
type builder struct {
	nodes   []LawNode
	ordinal int
}

func (b *builder) add(n LawNode) {
	n.Ordinal = b.ordinal
	b.ordinal++
	b.nodes = append(b.nodes, n)
}

// ParseLawXML parses 法令標準XML into the normalized, current-tree-only domain.
// It is pure with respect to the snapshot bytes (reproject-safe) and implements
// the shared eId contract.
func ParseLawXML(content []byte) (LawContent, error) {
	var x xmlLaw
	if err := xml.Unmarshal(content, &x); err != nil {
		return LawContent{}, err
	}

	b := &builder{}
	walkProvision(b, x.Body.MainProvision, "", scope{})
	for i, sp := range x.Body.SupplProvision {
		walkProvision(b, sp, "suppl_"+strconv.Itoa(i+1)+"__", scope{})
	}

	law := Law{
		LawNum:    strings.TrimSpace(x.LawNum),
		Title:     strings.TrimSpace(x.Body.LawTitle.Value),
		TitleKana: strings.TrimSpace(x.Body.LawTitle.Kana),
	}
	return LawContent{Law: law, Nodes: b.nodes}, nil
}

// walkProvision materializes the articles of one provision, prefixing every eId
// with eidPrefix ("" for 本則, "suppl_{k}__" for the k-th 附則).
func walkProvision(b *builder, p xmlProvision, eidPrefix string, sc scope) {
	for _, c := range p.Parts {
		walkContainer(b, c, eidPrefix, sc, containerOther)
	}
	for _, c := range p.Chapters {
		walkContainer(b, c, eidPrefix, sc, containerChapter)
	}
	for _, a := range p.Articles {
		walkArticle(b, a, eidPrefix, sc)
	}
}

// walkContainer descends a 編/章/節/款/目 wrapper, updating the chapter/section
// metadata carried onto the articles inside it.
func walkContainer(b *builder, c xmlContainer, eidPrefix string, sc scope, kind containerKind) {
	switch kind {
	case containerChapter:
		sc.chapterNum = c.Num
	case containerSection:
		sc.sectionNum = c.Num
	}
	for _, cc := range c.Chapters {
		walkContainer(b, cc, eidPrefix, sc, containerChapter)
	}
	for _, cc := range c.Sections {
		walkContainer(b, cc, eidPrefix, sc, containerSection)
	}
	for _, cc := range c.Subsections {
		walkContainer(b, cc, eidPrefix, sc, containerOther)
	}
	for _, cc := range c.Divisions {
		walkContainer(b, cc, eidPrefix, sc, containerOther)
	}
	for _, a := range c.Articles {
		walkArticle(b, a, eidPrefix, sc)
	}
}

func walkArticle(b *builder, a xmlArticle, eidPrefix string, sc scope) {
	isSuppl := eidPrefix != ""
	artEID := eidPrefix + "art_" + a.Num
	caption := strings.TrimSpace(a.Caption)

	// An article with no paragraphs still materializes, carrying its body text.
	sentence := ""
	if len(a.Paragraphs) == 0 {
		sentence = strings.TrimSpace(a.Title)
	}
	b.add(LawNode{
		EID:          artEID,
		ParentEID:    "",
		NodeType:     NodeArticle,
		Num:          a.Num,
		Caption:      caption,
		ChapterNum:   sc.chapterNum,
		SectionNum:   sc.sectionNum,
		IsSuppl:      isSuppl,
		SentenceText: sentence,
	})

	for i, p := range a.Paragraphs {
		num := p.Num
		if num == "" {
			num = strconv.Itoa(i + 1) // default to the 1-based ordinal
		}
		paraEID := artEID + "__para_" + num
		b.add(LawNode{
			EID:          paraEID,
			ParentEID:    artEID,
			NodeType:     NodeParagraph,
			Num:          num,
			ChapterNum:   sc.chapterNum,
			SectionNum:   sc.sectionNum,
			IsSuppl:      isSuppl,
			SentenceText: joinSentences(p.ParagraphSentence.Sentences),
		})
		for j, it := range p.Items {
			inum := it.Num
			if inum == "" {
				inum = strconv.Itoa(j + 1)
			}
			b.add(LawNode{
				EID:          paraEID + "__item_" + inum,
				ParentEID:    paraEID,
				NodeType:     NodeItem,
				Num:          inum,
				ChapterNum:   sc.chapterNum,
				SectionNum:   sc.sectionNum,
				IsSuppl:      isSuppl,
				SentenceText: joinSentences(it.ItemSentence.Sentences),
			})
		}
	}
}

func joinSentences(ss []string) string {
	parts := make([]string, 0, len(ss))
	for _, s := range ss {
		if t := strings.TrimSpace(s); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, "")
}
