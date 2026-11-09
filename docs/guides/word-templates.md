# Word templates for a type

A type can give its pages a Word template ([ADR-0167](../adr/0167-word-templates-give-a-type-its-document.md)). Such a page begins with the type's start text when it is made, reads as the template's document — cover, header, footer, the template's fonts and headings — and exports as a finished Word document: the template's own file with the page's text and fields in it. The page is edited in the same editor as every other page.

## Preparing a template in Word

Start from the organisation's `.dotx` (a `.docx` works too). Mark in it what the page fills in, with content controls:

1. Show the **Developer** tab (*File → Options → Customize Ribbon → Developer*).
2. Place the cursor where a field goes — on the cover, in the header or in the footer — and insert a **Plain Text Content Control**.
3. Open its **Properties** and set its **Tag**. The text the control shows until then is its placeholder; it is replaced on export.

| Tag | What it shows |
|---|---|
| `orbithek:title` | the page's title |
| `orbithek:key` | the page's key, e.g. `REP-12` |
| `orbithek:type`, `orbithek:state` | its type and its workflow state |
| `orbithek:space` | the name of its room |
| `orbithek:labels` | its labels, separated by commas |
| `orbithek:<field>` | the field of that name, e.g. `orbithek:owner` (a person shows by name) |
| `orbithek:body` | **where the page's text goes** |

Tags are matched without regard to case. Mark the place of the text with a **Rich Text Content Control** of its own paragraph, tagged `orbithek:body`, after the cover and any page break; whatever it holds in the template is replaced by the text. Without it, the text follows the template's content.

The page's headings, quotes, captions, lists and tables take the template's styles of the same names: *Heading 1* to *Heading 3*, *Quote*, *Caption*, *List Paragraph*, *Table Grid*, *Hyperlink*. They are found by their names, so a template made in a German Word, whose heading style is called *Überschrift 1*, works as it is. A style the template lacks is added with the product's own look. When the heading styles are numbered in the template (1, 1.1, …), the page's headings are numbered too.

A table of contents block becomes Word's own table of contents; Word offers to update it with page numbers when the document opens.

## What a template may not hold

On upload a template is checked and cleaned. Refused: macros (`.dotm`, `.docm`), ActiveX controls, embedded objects, and references to files or addresses outside the template other than hyperlinks — an external picture, for example, which Word would fetch when the document opens. Removed, and said so: the reference to the template a document was made from, which Word writes into templates and which names a file on the author's computer or a network share.

## Keeping templates

Administrators keep the templates under **Administration → Word templates**: an ID (lower-case letters, digits and dashes, e.g. `report`), a name and the file. The list shows each template's versions, the fields its controls show, whether it marks the place of the text, its header and footer, and which types and how many pages use it. **Upload a new version** keeps the new file as the next version; every version stays. A template is removed only when no type and no page uses it.

## Giving a type its template

Under **Administration → Types**, a type names its template by its ID and its start text in Markdown:

```json
{
  "name": "Report",
  "body": true,
  "template": "report",
  "fields": [{ "name": "owner", "kind": "user" }, { "name": "classification", "kind": "enum", "values": ["INTERNAL", "CONFIDENTIAL"] }],
  "startMarkdown": "# Background\n\n> [!NOTE]\n> Describe the situation.\n\n# Goals\n\n[toc]"
}
```

In the start text, a line `[toc]` stands for a table of contents, `[children]` for the list of child pages, `[figures]` and `[tables]` for the lists of figures and tables. Start texts hold text only: no pictures, embedded pages, processes or protected passages, which belong to one page.

## Versions and pages

A new page remembers the version of the template it was made with, and keeps exporting and reading with that version when a newer one is uploaded, so a document once sent out looks the same when it is exported again. The page says which version it is laid out after; when a newer one exists, whoever may edit the page can **Use version N**. A copy of a page keeps its original's version.

## Limits

- The reading view approximates Word: it does not break pages, wrap text around pictures or compute fields other than page numbers, and it shows pictures in PNG, JPEG and GIF only. The Word export keeps everything of the template.
- A page's text has three heading levels, tables without merged cells and one level of lists; footnotes, section breaks and landscape pages inside a text are not available.
- *Export → PDF* prints the page in the product's own printable form; a PDF in the template's design is saved from the Word document.
