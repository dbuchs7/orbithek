package wordtemplate

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"io"
	"slices"
	"testing"
)

// png1 is a picture of one pixel.
var png1, _ = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8DwHwAFBQIAX8jx0gAAAABJRU5ErkJggg==")

const xmlHead = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n"

// fixture are the parts of a template as a German Word writes one: style
// IDs in German, names in English, a cover with the title in a control,
// a page break, a heading, the author inline, the place of the text, a
// header with the logo and the page number, a footer with the number of
// pages, a first page of its own, and the attached template's reference.
func fixture() map[string]string {
	return map[string]string{
		"[Content_Types].xml": xmlHead + `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/>` +
			`<Default Extension="png" ContentType="image/png"/>` +
			`<Override PartName="/word/document.xml" ContentType="` + templateMain + `"/>` +
			`<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>` +
			`<Override PartName="/word/numbering.xml" ContentType="` + numberingCT + `"/>` +
			`<Override PartName="/word/settings.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml"/>` +
			`<Override PartName="/word/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>` +
			`<Override PartName="/word/header1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml"/>` +
			`<Override PartName="/word/footer1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.footer+xml"/>` +
			`<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>` +
			`</Types>`,
		"_rels/.rels": xmlHead + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>` +
			`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>` +
			`</Relationships>`,
		"docProps/core.xml": xmlHead + `<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Vorlage</dc:title><dc:creator>Muster</dc:creator></cp:coreProperties>`,
		"word/_rels/document.xml.rels": xmlHead + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>` +
			`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering" Target="numbering.xml"/>` +
			`<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/settings" Target="settings.xml"/>` +
			`<Relationship Id="rId4" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="theme/theme1.xml"/>` +
			`<Relationship Id="rId5" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header1.xml"/>` +
			`<Relationship Id="rId6" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer" Target="footer1.xml"/>` +
			`<Relationship Id="rId7" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/image1.png"/>` +
			`<Relationship Id="rId8" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://example.org/" TargetMode="External"/>` +
			`</Relationships>`,
		"word/_rels/header1.xml.rels": xmlHead + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/image1.png"/>` +
			`</Relationships>`,
		"word/_rels/settings.xml.rels": xmlHead + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/attachedTemplate" Target="file:///\\server\share\Bericht.dotx" TargetMode="External"/>` +
			`</Relationships>`,
		"word/media/image1.png": string(png1),
		"word/settings.xml":     xmlHead + `<w:settings xmlns:w="` + nsW + `" xmlns:r="` + nsR + `"><w:zoom w:percent="100"/><w:attachedTemplate r:id="rId1"/><w:defaultTabStop w:val="708"/></w:settings>`,
		"word/theme/theme1.xml": xmlHead + `<a:theme xmlns:a="` + nsA + `" name="Office"><a:themeElements><a:clrScheme name="Office">` +
			`<a:dk1><a:sysClr val="windowText" lastClr="000000"/></a:dk1><a:lt1><a:sysClr val="window" lastClr="FFFFFF"/></a:lt1>` +
			`<a:dk2><a:srgbClr val="44546A"/></a:dk2><a:lt2><a:srgbClr val="E7E6E6"/></a:lt2><a:accent1><a:srgbClr val="DC0018"/></a:accent1>` +
			`<a:hlink><a:srgbClr val="0563C1"/></a:hlink></a:clrScheme>` +
			`<a:fontScheme name="Office"><a:majorFont><a:latin typeface="Frutiger Bold"/></a:majorFont><a:minorFont><a:latin typeface="Frutiger"/></a:minorFont></a:fontScheme>` +
			`</a:themeElements></a:theme>`,
		"word/styles.xml": xmlHead + `<w:styles xmlns:w="` + nsW + `">` +
			`<w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:asciiTheme="minorHAnsi" w:hAnsiTheme="minorHAnsi"/><w:sz w:val="20"/></w:rPr></w:rPrDefault>` +
			`<w:pPrDefault><w:pPr><w:spacing w:after="120" w:line="264" w:lineRule="auto"/></w:pPr></w:pPrDefault></w:docDefaults>` +
			`<w:style w:type="paragraph" w:default="1" w:styleId="Standard"><w:name w:val="Normal"/><w:qFormat/></w:style>` +
			`<w:style w:type="paragraph" w:styleId="Titel"><w:name w:val="Title"/><w:basedOn w:val="Standard"/><w:pPr><w:jc w:val="center"/></w:pPr><w:rPr><w:rFonts w:asciiTheme="majorHAnsi"/><w:sz w:val="56"/><w:color w:val="FFFFFF" w:themeColor="accent1" w:themeShade="BF"/></w:rPr></w:style>` +
			`<w:style w:type="paragraph" w:styleId="berschrift1"><w:name w:val="heading 1"/><w:basedOn w:val="Standard"/><w:next w:val="Standard"/>` +
			`<w:pPr><w:keepNext/><w:numPr><w:numId w:val="1"/></w:numPr><w:spacing w:before="480" w:after="120"/><w:pBdr><w:bottom w:val="single" w:sz="8" w:space="4" w:color="DC0018"/></w:pBdr><w:outlineLvl w:val="0"/></w:pPr><w:rPr><w:b/><w:sz w:val="32"/></w:rPr></w:style>` +
			`<w:style w:type="paragraph" w:styleId="berschrift2"><w:name w:val="heading 2"/><w:basedOn w:val="berschrift1"/>` +
			`<w:pPr><w:numPr><w:ilvl w:val="1"/><w:numId w:val="1"/></w:numPr><w:pBdr><w:bottom w:val="nil"/></w:pBdr></w:pPr><w:rPr><w:sz w:val="26"/><w:i/></w:rPr></w:style>` +
			`<w:style w:type="paragraph" w:styleId="Zitat"><w:name w:val="Quote"/><w:basedOn w:val="Standard"/><w:pPr><w:ind w:left="567" w:hanging="283"/></w:pPr><w:rPr><w:i/><w:caps/></w:rPr></w:style>` +
			`<w:style w:type="paragraph" w:styleId="Kopfzeile"><w:name w:val="header"/><w:basedOn w:val="Standard"/><w:pPr><w:tabs><w:tab w:val="right" w:pos="9072"/></w:tabs></w:pPr><w:rPr><w:sz w:val="16"/></w:rPr></w:style>` +
			`<w:style w:type="character" w:styleId="Hyperlink"><w:name w:val="Hyperlink"/><w:rPr><w:color w:val="0563C1" w:themeColor="hyperlink"/><w:u w:val="single"/></w:rPr></w:style>` +
			`<w:style w:type="character" w:styleId="Fett"><w:name w:val="Strong"/><w:rPr><w:b/><w:highlight w:val="yellow"/></w:rPr></w:style>` +
			`<w:style w:type="table" w:styleId="Tabellenraster"><w:name w:val="Table Grid"/><w:tblPr><w:tblBorders><w:top w:val="single" w:sz="4"/></w:tblBorders></w:tblPr></w:style>` +
			`</w:styles>`,
		"word/numbering.xml": xmlHead + `<w:numbering xmlns:w="` + nsW + `">` +
			`<w:abstractNum w:abstractNumId="3"><w:multiLevelType w:val="multilevel"/>` +
			`<w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%1"/></w:lvl>` +
			`<w:lvl w:ilvl="1"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%1.%2"/></w:lvl></w:abstractNum>` +
			`<w:num w:numId="1"><w:abstractNumId w:val="3"/></w:num><w:num w:numId="4"><w:abstractNumId w:val="3"/></w:num>` +
			`</w:numbering>`,
		"word/header1.xml": xmlHead + `<w:hdr xmlns:w="` + nsW + `" xmlns:r="` + nsR + `" xmlns:wp="` + nsWP + `" xmlns:a="` + nsA + `" xmlns:pic="` + nsPic + `">` +
			`<w:p><w:pPr><w:pStyle w:val="Kopfzeile"/></w:pPr>` +
			`<w:r><w:drawing><wp:anchor behindDoc="0"><wp:positionH relativeFrom="margin"><wp:align>right</wp:align></wp:positionH><wp:extent cx="1270000" cy="635000"/>` +
			`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic><pic:blipFill><a:blip r:embed="rId1"/></pic:blipFill></pic:pic></a:graphicData></a:graphic></wp:anchor></w:drawing></w:r>` +
			`<w:sdt><w:sdtPr><w:tag w:val="orbithek:title"/><w:showingPlcHdr/></w:sdtPr><w:sdtContent><w:r><w:rPr><w:b/></w:rPr><w:t>[Titel]</w:t></w:r></w:sdtContent></w:sdt>` +
			`<w:r><w:tab/><w:t xml:space="preserve">Seite </w:t></w:r>` +
			`<w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText xml:space="preserve"> PAGE </w:instrText></w:r><w:r><w:fldChar w:fldCharType="separate"/></w:r><w:r><w:t>1</w:t></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r>` +
			`</w:p></w:hdr>`,
		"word/footer1.xml": xmlHead + `<w:ftr xmlns:w="` + nsW + `"><w:p><w:r><w:t xml:space="preserve">von </w:t></w:r><w:fldSimple w:instr=" NUMPAGES "><w:r><w:t>3</w:t></w:r></w:fldSimple>` +
			`<w:r><w:t xml:space="preserve"> · </w:t></w:r><w:sdt><w:sdtPr><w:tag w:val="ORBITHEK:Klassifizierung"/></w:sdtPr><w:sdtContent><w:r><w:t>INTERN</w:t></w:r></w:sdtContent></w:sdt></w:p></w:ftr>`,
		"word/document.xml": xmlHead + `<w:document xmlns:w="` + nsW + `" xmlns:r="` + nsR + `" xmlns:wp="` + nsWP + `" xmlns:a="` + nsA + `" xmlns:pic="` + nsPic + `"><w:body>` +
			`<w:sdt><w:sdtPr><w:alias w:val="Titel"/><w:tag w:val="orbithek:title"/><w:dataBinding w:xpath="/ns1:coreProperties[1]/ns0:title[1]" w:storeItemID="{6C3C8BC8-F283-45AE-878A-BAB7291924A1}"/><w:showingPlcHdr/></w:sdtPr>` +
			`<w:sdtContent><w:p><w:pPr><w:pStyle w:val="Titel"/></w:pPr><w:r><w:rPr><w:color w:val="808080"/></w:rPr><w:t>[Titel des Berichts]</w:t></w:r></w:p></w:sdtContent></w:sdt>` +
			`<w:p><w:r><w:t xml:space="preserve">Verfasst von </w:t></w:r><w:sdt><w:sdtPr><w:tag w:val="orbithek:author"/></w:sdtPr><w:sdtContent><w:r><w:rPr><w:i/></w:rPr><w:t>[Autor]</w:t></w:r></w:sdtContent></w:sdt>` +
			`<w:r><w:t xml:space="preserve"> am </w:t></w:r><w:sdt><w:sdtPr><w:tag w:val="orbithek:unknown"/></w:sdtPr><w:sdtContent><w:r><w:t>bleibt</w:t></w:r></w:sdtContent></w:sdt></w:p>` +
			`<w:p><w:r><w:br w:type="page"/></w:r></w:p>` +
			`<w:p><w:pPr><w:pStyle w:val="berschrift1"/></w:pPr><w:r><w:t>Inhalt</w:t></w:r></w:p>` +
			`<w:tbl><w:tblPr><w:tblStyle w:val="Tabellenraster"/></w:tblPr><w:tr><w:tc><w:tcPr><w:tcW w:w="2000" w:type="dxa"/><w:shd w:val="clear" w:fill="F2F2F2"/></w:tcPr><w:p><w:r><w:t>Zelle</w:t></w:r></w:p></w:tc>` +
			`<w:tc><w:tcPr><w:gridSpan w:val="2"/></w:tcPr><w:p><w:hyperlink r:id="rId8"><w:r><w:rPr><w:rStyle w:val="Fett"/></w:rPr><w:t>Link</w:t></w:r></w:hyperlink></w:p></w:tc></w:tr></w:tbl>` +
			`<w:sdt><w:sdtPr><w:tag w:val="orbithek:body"/></w:sdtPr><w:sdtContent><w:p><w:r><w:t>Hier steht der Text.</w:t></w:r></w:p></w:sdtContent></w:sdt>` +
			`<w:p><w:r><w:t>Schluss</w:t></w:r></w:p>` +
			`<w:sectPr><w:headerReference w:type="default" r:id="rId5"/><w:footerReference w:type="default" r:id="rId6"/><w:headerReference w:type="first" r:id="rId5"/>` +
			`<w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1701" w:right="1134" w:bottom="1134" w:left="1418" w:header="567" w:footer="567" w:gutter="0"/><w:titlePg/></w:sectPr>` +
			`</w:body></w:document>`,
	}
}

// pack zips parts in a stable order.
func pack(t *testing.T, parts map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	names := []string{"[Content_Types].xml", "_rels/.rels"}
	for name := range parts {
		if name != names[0] && name != names[1] {
			names = append(names, name)
		}
	}
	slices.Sort(names[2:])
	for _, name := range names {
		data, ok := parts[name]
		if !ok {
			continue
		}
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(w, data); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// unpack reads a written document's parts.
func unpack(t *testing.T, data []byte) map[string]string {
	t.Helper()
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, f := range z.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(rc)
		rc.Close()
		out[f.Name] = string(b)
	}
	return out
}

func parsed(t *testing.T, parts map[string]string) *Template {
	t.Helper()
	tp, err := Parse(pack(t, parts), 10<<20)
	if err != nil {
		t.Fatal(err)
	}
	return tp
}
