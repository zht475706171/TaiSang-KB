//go:build gen

// gen.go generates minimal OOXML fixtures (docx/xlsx/pptx) for parser tests
// using only the Go standard library (archive/zip + hand-written XML).
//
// We avoid unioffice here because v1.39.0 is a commercial product that
// fails SaveToFile in unlicensed mode ("unioffice license required").
// The generated files are intentionally minimal but valid enough that a
// real Office reader (and our future parser) can extract their text.
//
// Run: go run -tags gen testdata/parser/gen.go
package main

import (
	"archive/zip"
	"log"
	"os"
	"path/filepath"
)

func main() {
	dir := "testdata/parser"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatal(err)
	}
	if err := writeZip(filepath.Join(dir, "sample.docx"), docxFiles()); err != nil {
		log.Fatal("docx:", err)
	}
	if err := writeZip(filepath.Join(dir, "sample.xlsx"), xlsxFiles()); err != nil {
		log.Fatal("xlsx:", err)
	}
	if err := writeZip(filepath.Join(dir, "sample.pptx"), pptxFiles()); err != nil {
		log.Fatal("pptx:", err)
	}
	log.Println("fixtures generated in", dir)
}

// writeZip writes a zip file with the given (name -> content) entries.
func writeZip(path string, files map[string]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := zip.NewWriter(f)
	defer w.Close()
	for name, body := range files {
		zf, err := w.Create(name)
		if err != nil {
			return err
		}
		if _, err := zf.Write([]byte(body)); err != nil {
			return err
		}
	}
	return nil
}

const contentTypesDocx = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`

const relsRoot = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`

const relsDocx = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
</Relationships>`

const docxDocument = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:body>
<w:p><w:r><w:t>DOCX 测试段落一。</w:t></w:r></w:p>
<w:p><w:r><w:t>第二段落 with English.</w:t></w:r></w:p>
</w:body>
</w:document>`

func docxFiles() map[string]string {
	return map[string]string{
		"[Content_Types].xml":        contentTypesDocx,
		"_rels/.rels":                relsRoot,
		"word/document.xml":          docxDocument,
		"word/_rels/document.xml.rels": relsDocx,
	}
}

const contentTypesXlsx = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
<Override PartName="/xl/sharedStrings.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml"/>
</Types>`

const relsXlsx = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`

const xlsxWorkbook = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets>
</workbook>`

const xlsxWorkbookRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings" Target="sharedStrings.xml"/>
</Relationships>`

// sharedStrings table: index 0 = "XLSX 单元格 A1", index 1 = "第二行 B1"
const xlsxSharedStrings = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="2" uniqueCount="2">
<si><t>XLSX 单元格 A1</t></si>
<si><t>第二行 B1</t></si>
</sst>`

// sheet1: row1 cell A1 = shared string 0, row2 cell A1 = shared string 1
const xlsxSheet1 = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<sheetData>
<row r="1"><c r="A1" t="s"><v>0</v></c></row>
<row r="2"><c r="A1" t="s"><v>1</v></c></row>
</sheetData>
</worksheet>`

func xlsxFiles() map[string]string {
	return map[string]string{
		"[Content_Types].xml":          contentTypesXlsx,
		"_rels/.rels":                  relsXlsx,
		"xl/workbook.xml":              xlsxWorkbook,
		"xl/_rels/workbook.xml.rels":   xlsxWorkbookRels,
		"xl/worksheets/sheet1.xml":     xlsxSheet1,
		"xl/sharedStrings.xml":         xlsxSharedStrings,
	}
}

const contentTypesPptx = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
<Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
</Types>`

const relsPptx = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
</Relationships>`

const pptxPresentationRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
</Relationships>`

const pptxPresentation = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:presentation xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
<sldIdLst><sldId id="1" r:id="rId1"/></sldIdLst>
</p:presentation>`

// slide1: one text box with one paragraph containing one run.
const pptxSlide1 = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
<p:cSld><p:spTree>
<p:sp>
<p:nvSpPr><p:cNvPr id="1" name="TextBox 1"/><p:cNvSpPr><a:spLocks noText="1"/></p:cNvSpPr><p:nvPr/></p:nvSpPr>
<p:spPr/>
<p:txBody>
<a:bodyPr/>
<a:lstStyle/>
<a:p><a:r><a:t>PPTX 幻灯片一内容</a:t></a:r></a:p>
</p:txBody>
</p:sp>
</p:spTree></p:cSld>
</p:sld>`

func pptxFiles() map[string]string {
	return map[string]string{
		"[Content_Types].xml":              contentTypesPptx,
		"_rels/.rels":                      relsPptx,
		"ppt/presentation.xml":             pptxPresentation,
		"ppt/_rels/presentation.xml.rels":  pptxPresentationRels,
		"ppt/slides/slide1.xml":            pptxSlide1,
	}
}