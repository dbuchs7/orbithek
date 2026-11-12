#!/bin/sh
# Rebuilds the vendored bundles in ../static/vendor from the pinned packages:
# prosemirror.js, the editor (ADR-0032), excalidraw.js with its stylesheet,
# chunks and fonts, the drawing editor (ADR-0053, ADR-0126), and
# bpmn-viewer.js and bpmn-modeler.js with their stylesheets and icon font in
# bpmn/, the BPMN viewer and modeler (ADR-0129), and form-viewer.js and
# dmn-viewer.js with their stylesheets in form/ and dmn/, the viewers of
# forms and decisions (ADR-0144), and pdf.js and pdf.worker.js with their
# fonts, character maps and image decoders in pdfjs/, the PDF reader
# (ADR-0208).
# Their license files name the packages each bundle takes code from
# (bundled.mjs). Run after changing a version in package.json:
#
#   cd internal/web/vendor && npm ci && sh build.sh
#
# Commit the bundles, their license files and package-lock.json together,
# and update THIRD_PARTY_NOTICES.md; internal/web tests check both.
set -eu
out=../static/vendor
npx esbuild entry.js --bundle --format=esm --target=es2020 --minify \
  --legal-comments=none --metafile=editor-meta.json --outfile="$out/prosemirror.js"
{
  echo "The bundle prosemirror.js contains these packages, each under the MIT license below."
  echo
  node bundled.mjs editor-meta.json
  echo
  cat node_modules/prosemirror-model/LICENSE
} > "$out/prosemirror.LICENSE.txt"
rm editor-meta.json

# The drawing bundle (ADR-0053, ADR-0126): excalidraw.js and excalidraw.css,
# and the chunks they load when needed in excalidraw-assets/. Excalidraw's
# Mermaid import is replaced (no-mermaid.js), and of its translations only
# the languages of the UI are kept (English is built in).
rm -rf "$out/excalidraw-assets" "$out/fonts" "$out"/excalidraw.*
npx esbuild draw-entry.js --bundle --splitting --format=esm --target=es2020 --minify \
  --legal-comments=none --define:process.env.NODE_ENV='"production"' \
  --define:process.env.IS_PREACT='"false"' --alias:@excalidraw/mermaid-to-excalidraw=./no-mermaid.js --conditions=production \
  --external:'./fonts/*' --metafile=draw-meta.json --entry-names=excalidraw --chunk-names='excalidraw-assets/[name]-[hash]' --outdir="$out"
for f in node_modules/@excalidraw/excalidraw/dist/prod/locales/*.js; do
  name=$(basename "$f" .js)
  case "$name" in de-DE-*|en-*|percentages-*) ;; *) rm "$out/excalidraw-assets/$name"-*.js ;; esac
done
# Excalidraw lists a CDN after the path the page sets for its fonts. The
# content security policy blocks it, but browsers still try it; the bundle
# names the installation's own path only.
# A drawing exported as SVG (ADR-0173) carries the fonts its text needs.
# Excalidraw first cuts them down to the glyphs used, with a decoder that
# compiles code from strings; the drawing page's policy forbids that, so the
# attempt fails, reports a policy violation and then embeds the whole font
# file anyway. The bundle embeds the font files at once, without trying.
# The build fails if the code either change replaces is not found exactly
# once, as after an update that changed it.
node -e '
  const fs = require("fs");
  const dir = process.argv[1];
  const id = "[A-Za-z_$][\\w$]*";
  const patches = [
    ["the CDN fallback for fonts", new RegExp(`return (${id})\\.push\\(new URL\\(${id},${id}\\.ASSETS_FALLBACK_URL\\)\\),\\1\\}`, "g"),
      (_, list) => `return ${list}}`],
    ["the cutting down of fonts", new RegExp(`=async\\((${id}),${id}\\)=>\\{let\\{Commands:${id},subsetToBase64:${id},toBase64:(${id})\\}=await ${id}\\(\\);return `, "g"),
      (m, font, toBase64) => `${m}${toBase64}(${font});return `],
  ];
  for (const [name, re, to] of patches) {
    let n = 0;
    for (const f of fs.readdirSync(dir)) {
      const p = dir + "/" + f;
      const src = fs.readFileSync(p, "utf8");
      const out = src.replace(re, (...m) => { n++; return to(...m); });
      if (out !== src) fs.writeFileSync(p, out);
    }
    if (n !== 1) { console.error(`${name} was found ${n} times, not once`); process.exit(1); }
  }
' "$out/excalidraw-assets"
# Its fonts come from this server, not from a CDN: the page sets
# EXCALIDRAW_ASSET_PATH, and the stylesheet names them relative to itself.
# Xiaolai, the fallback for Chinese, Japanese and Korean (13 MB), is left out.
dist=node_modules/@excalidraw/excalidraw/dist/prod
mkdir -p "$out/fonts"
for d in "$dist"/fonts/*; do
  [ "$(basename "$d")" = Xiaolai ] || cp -r "$d" "$out/fonts/"
done
# Every package the bundle took code from, with its version, license and
# license text.
{
  echo "The bundle excalidraw.js, excalidraw.css and the files in excalidraw-assets/"
  echo "contain code of these packages, each under the license named; their license"
  echo "texts follow. The fonts in fonts/ (Excalifont, Virgil, Cascadia, Assistant,"
  echo "Liberation Sans, Lilita One and Nunito) are under the SIL Open Font License"
  echo "1.1, Comic Shanns under the MIT license."
  echo
  node bundled.mjs draw-meta.json --texts
} > "$out/excalidraw.LICENSE.txt"
rm draw-meta.json

# The BPMN viewer and modeler (ADR-0129): two bundles, so a page that shows a
# diagram does not load the modeler, with bpmn-js's stylesheets and icon font
# in bpmn/, copied as published. The watermark code is part of the bundles,
# unchanged, as the bpmn.io license requires. The modeler holds all the
# viewer does, so its account names the packages of both.
npx esbuild bpmn-entry.js --bundle --format=esm --target=es2020 --minify \
  --legal-comments=none --outfile="$out/bpmn-viewer.js"
npx esbuild bpmn-modeler-entry.js --bundle --format=esm --target=es2020 --minify \
  --legal-comments=none --metafile=bpmn-meta.json --outfile="$out/bpmn-modeler.js"
bpmn=node_modules/bpmn-js/dist/assets
rm -rf "$out/bpmn"
mkdir -p "$out/bpmn/bpmn-font/css" "$out/bpmn/bpmn-font/font"
cp "$bpmn/bpmn-js.css" "$bpmn/diagram-js.css" "$out/bpmn/"
cp "$bpmn/bpmn-font/css/bpmn.css" "$out/bpmn/bpmn-font/css/"
cp "$bpmn/bpmn-font/font/bpmn.woff2" "$bpmn/bpmn-font/font/bpmn.woff" "$out/bpmn/bpmn-font/font/"
{
  echo "The bundles bpmn-viewer.js and bpmn-modeler.js and the files in bpmn/ contain"
  echo "code of these packages, each under the license named; their license texts"
  echo "follow. bpmn-js is under the bpmn.io license: the MIT terms with the condition"
  echo "that the code displaying the bpmn.io watermark in rendered diagrams is neither"
  echo "removed nor changed, and that the watermark stays fully visible. path-"
  echo "intersection carries a notice for the code it derives from Snap.svg, at the end."
  echo
  node bundled.mjs bpmn-meta.json --texts
  echo; echo "== path-intersection (NOTICE)"; echo
  cat node_modules/path-intersection/NOTICE
} > "$out/bpmn.LICENSE.txt"
rm bpmn-meta.json

# The form viewer and the DMN viewer (ADR-0144): form-viewer.js, form-js's
# viewer, with its stylesheet in form/, and dmn-viewer.js, dmn-js's
# navigated viewer, with its stylesheets and icon font in dmn/, copied as
# published. As for bpmn-js, the watermark code is part of the bundles,
# unchanged, as the bpmn.io license requires.
npx esbuild form-entry.js --bundle --format=esm --target=es2020 --minify \
  --legal-comments=none --define:process.env.NODE_ENV='"production"' \
  --metafile=form-meta.json --outfile="$out/form-viewer.js"
rm -rf "$out/form"
mkdir -p "$out/form"
cp node_modules/@bpmn-io/form-js-viewer/dist/assets/form-js.css "$out/form/"
{
  echo "The bundle form-viewer.js and the file in form/ contain code of these"
  echo "packages, each under the license named; their license texts follow."
  echo "@bpmn-io/form-js-viewer is under the bpmn.io license: the MIT terms with the"
  echo "condition that the code displaying the bpmn.io watermark in rendered forms is"
  echo "neither removed nor changed, and that the watermark stays fully visible."
  echo
  node bundled.mjs form-meta.json --texts
} > "$out/form.LICENSE.txt"
rm form-meta.json

npx esbuild dmn-entry.js --bundle --format=esm --target=es2020 --minify \
  --legal-comments=none --define:process.env.NODE_ENV='"production"' \
  --metafile=dmn-meta.json --outfile="$out/dmn-viewer.js"
dmn=node_modules/dmn-js/dist/assets
rm -rf "$out/dmn"
mkdir -p "$out/dmn/dmn-font/css" "$out/dmn/dmn-font/font"
cp "$dmn/diagram-js.css" "$dmn/dmn-js-shared.css" "$dmn/dmn-js-drd.css" "$dmn/dmn-js-decision-table.css" \
  "$dmn/dmn-js-literal-expression.css" "$dmn/dmn-js-boxed-expression.css" "$out/dmn/"
cp "$dmn/dmn-font/css/dmn.css" "$out/dmn/dmn-font/css/"
cp "$dmn/dmn-font/font/dmn.woff2" "$dmn/dmn-font/font/dmn.woff" "$out/dmn/dmn-font/font/"
{
  echo "The bundle dmn-viewer.js and the files in dmn/ contain code of these packages,"
  echo "each under the license named; their license texts follow. dmn-js is under the"
  echo "bpmn.io license: the MIT terms with the condition that the code displaying the"
  echo "bpmn.io watermark in rendered diagrams is neither removed nor changed, and that"
  echo "the watermark stays fully visible. path-intersection carries a notice for"
  echo "the code it derives from Snap.svg, at the end."
  echo
  node bundled.mjs dmn-meta.json --texts
  echo; echo "== path-intersection (NOTICE)"; echo
  cat node_modules/path-intersection/NOTICE
} > "$out/dmn.LICENSE.txt"
rm dmn-meta.json

# The PDF reader (ADR-0208): pdf.js, its worker and what the worker loads
# when a document needs it, taken unchanged from pdfjs-dist's prebuilt
# modules, renamed to .js. Of its directories: standard_fonts/ without the
# Liberation fonts, whose license is the GPL version 2 with a font
# exception (the browser's own sans-serif stands in for them), cmaps/ for
# the character maps of East Asian text, and of wasm/ only the decoders of
# JPEG 2000 and JBIG2 images compiled to JavaScript: the UI's content
# security policy runs no WebAssembly, and the reader asks for those.
pdf=node_modules/pdfjs-dist
rm -rf "$out/pdfjs" "$out"/pdf.*
mkdir -p "$out/pdfjs/standard_fonts" "$out/pdfjs/cmaps" "$out/pdfjs/wasm"
cp "$pdf/build/pdf.min.mjs" "$out/pdf.js"
cp "$pdf/build/pdf.worker.min.mjs" "$out/pdf.worker.js"
cp "$pdf"/standard_fonts/Foxit*.pfb "$pdf/standard_fonts/LICENSE_FOXIT" "$out/pdfjs/standard_fonts/"
cp "$pdf"/cmaps/*.bcmap "$pdf/cmaps/LICENSE" "$out/pdfjs/cmaps/"
cp "$pdf/wasm/openjpeg_nowasm_fallback.js" "$pdf/wasm/jbig2_nowasm_fallback.js" "$out/pdfjs/wasm/"
{
  echo "The files pdf.js and pdf.worker.js and the files in pdfjs/ are taken unchanged"
  echo "from this package, under the license named; its license text follows, then"
  echo "those of the parts it carries: the JPEG 2000 decoder (OpenJPEG, BSD 2-clause)"
  echo "and the JBIG2 decoder (PDFium, BSD 3-clause) in pdfjs/wasm/, compiled by"
  echo "Mozilla, the fonts by Foxit (PDFium, BSD 3-clause) in pdfjs/standard_fonts/"
  echo "and the character maps by Adobe (BSD 3-clause) in pdfjs/cmaps/."
  echo
  echo "pdfjs-dist $(node -p 'require("./'"$pdf"'/package.json").version') Apache-2.0"
  echo
  echo "== pdfjs-dist (Apache-2.0)"; echo; cat "$pdf/LICENSE"
  echo; echo "== OpenJPEG"; echo; cat "$pdf/wasm/LICENSE_OPENJPEG"; echo; cat "$pdf/wasm/LICENSE_PDFJS_OPENJPEG"
  echo; echo "== JBIG2 decoder of PDFium"; echo; cat "$pdf/wasm/LICENSE_JBIG2"; echo; cat "$pdf/wasm/LICENSE_PDFJS_JBIG2"
  echo; echo "== Foxit fonts"; echo; cat "$pdf/standard_fonts/LICENSE_FOXIT"
  echo; echo "== Adobe CMaps"; echo; cat "$pdf/cmaps/LICENSE"
} > "$out/pdf.LICENSE.txt"
