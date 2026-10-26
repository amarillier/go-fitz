//go:build cgo && !nocgo

package fitz

/*
#include <mupdf/fitz.h>
#include <stdlib.h>

const char *fz_version = FZ_VERSION;
#if defined(_WIN32)
	typedef unsigned long long store;
#else
	typedef unsigned long store;
#endif

fz_document *open_document(fz_context *ctx, const char *filename) {
	fz_document *doc;

	fz_try(ctx) {
		doc = fz_open_document(ctx, filename);
	}
	fz_catch(ctx) {
		return NULL;
	}

	return doc;
}

fz_document *open_document_with_stream(fz_context *ctx, const char *magic, fz_stream *stream) {
	fz_document *doc;

	fz_try(ctx) {
		doc = fz_open_document_with_stream(ctx, magic, stream);
	}
	fz_catch(ctx) {
		return NULL;
	}

	return doc;
}

fz_page *load_page(fz_context *ctx, fz_document *doc, int number) {
	fz_page *page;

	fz_try(ctx) {
		page = fz_load_page(ctx, doc, number);
	}
	fz_catch(ctx) {
		return NULL;
	}

	return page;
}

int run_page_contents(fz_context *ctx, fz_page *page, fz_device *dev, fz_matrix transform, fz_cookie *cookie) {
	fz_try(ctx) {
		fz_run_page_contents(ctx, page, dev, transform, cookie);
	}
	fz_catch(ctx) {
		return 0;
	}

	return 1;
}

// run_page runs a page's own content stream AND its annotations' real
// appearance streams (fz_run_page = fz_run_page_contents + fz_run_page_annots
// in one call, per MuPDF's own fitz/document.h) — added for KrankyBearReader,
// which needs annotations (Stamp especially, whose appearance is arbitrary
// vector art with no fixed geometry to hand-roll a draw routine from) painted
// the same way a real PDF viewer paints them, not composited by hand
// afterward. Deliberately not touching run_page_contents above: some callers
// (e.g. this app's own annotation-overlay code, still used for
// click-to-select and recoloring) want the content-only render and paint
// their own overlays on top of it.
int run_page(fz_context *ctx, fz_page *page, fz_device *dev, fz_matrix transform, fz_cookie *cookie) {
	fz_try(ctx) {
		fz_run_page(ctx, page, dev, transform, cookie);
	}
	fz_catch(ctx) {
		return 0;
	}

	return 1;
}

// kbr_stext_char is one entry of stext_chars' flat output: a character (or a
// '\n' pseudo-entry mirroring the newlines fz_print_stext_page_as_text emits,
// so entry i lines up with rune i of Document.Text's string).
typedef struct {
	int c;
	int line;
	float quad[8]; // ul.x ul.y ur.x ur.y ll.x ll.y lr.x lr.y
	float size;
} kbr_stext_char;

// stext_chars extracts a page's characters with their exact quads — added
// for KrankyBearReader's text-snapped highlights and exact search boxes,
// which need per-character positions Document.Text and Document.HTML (line
// level only) don't expose. Runs the page's content only (like Text) with
// default options, then walks blocks/lines/chars, appending a '\n' entry
// after every line and one more after every text block — the same layout
// fz_print_stext_page_as_text writes. Returns the entry count and a malloc'd
// array in *out (release with stext_free), or -1 on error.
int stext_chars(fz_context *ctx, fz_page *page, kbr_stext_char **out) {
	fz_stext_page *text = NULL;
	fz_device *dev = NULL;
	kbr_stext_char *arr = NULL;
	int n = 0, cap = 0, line_no = 0;
	fz_stext_options opts;
	fz_cookie cookie;

	memset(&opts, 0, sizeof(opts));
	memset(&cookie, 0, sizeof(cookie));
	fz_var(text);
	fz_var(dev);
	fz_var(arr);
	fz_var(n);
	fz_var(cap);
	fz_var(line_no);

	fz_try(ctx) {
		fz_stext_block *b;
		fz_stext_line *l;
		fz_stext_char *ch;

		text = fz_new_stext_page(ctx, fz_bound_page(ctx, page));
		dev = fz_new_stext_device(ctx, text, &opts);
		fz_run_page_contents(ctx, page, dev, fz_identity, &cookie);
		fz_close_device(ctx, dev);

		for (b = text->first_block; b; b = b->next) {
			if (b->type != FZ_STEXT_BLOCK_TEXT)
				continue;
			for (l = b->u.t.first_line; l; l = l->next) {
				for (ch = l->first_char; ch; ch = ch->next) {
					if (n == cap) {
						cap = cap ? cap * 2 : 256;
						arr = realloc(arr, cap * sizeof(kbr_stext_char));
						if (!arr)
							fz_throw(ctx, FZ_ERROR_GENERIC, "out of memory");
					}
					arr[n].c = ch->c;
					arr[n].line = line_no;
					arr[n].quad[0] = ch->quad.ul.x; arr[n].quad[1] = ch->quad.ul.y;
					arr[n].quad[2] = ch->quad.ur.x; arr[n].quad[3] = ch->quad.ur.y;
					arr[n].quad[4] = ch->quad.ll.x; arr[n].quad[5] = ch->quad.ll.y;
					arr[n].quad[6] = ch->quad.lr.x; arr[n].quad[7] = ch->quad.lr.y;
					arr[n].size = ch->size;
					n++;
				}
				// the '\n' ending this line (zero quad)
				if (n == cap) {
					cap = cap ? cap * 2 : 256;
					arr = realloc(arr, cap * sizeof(kbr_stext_char));
					if (!arr)
						fz_throw(ctx, FZ_ERROR_GENERIC, "out of memory");
				}
				memset(&arr[n], 0, sizeof(kbr_stext_char));
				arr[n].c = '\n';
				arr[n].line = line_no;
				n++;
				line_no++;
			}
			// the blank line fz_print_stext_page_as_text writes after a block
			if (n == cap) {
				cap = cap ? cap * 2 : 256;
				arr = realloc(arr, cap * sizeof(kbr_stext_char));
				if (!arr)
					fz_throw(ctx, FZ_ERROR_GENERIC, "out of memory");
			}
			memset(&arr[n], 0, sizeof(kbr_stext_char));
			arr[n].c = '\n';
			arr[n].line = line_no - 1;
			n++;
		}
	}
	fz_always(ctx) {
		fz_drop_device(ctx, dev);
		fz_drop_stext_page(ctx, text);
	}
	fz_catch(ctx) {
		free(arr);
		return -1;
	}

	*out = arr;
	return n;
}

void stext_free(kbr_stext_char *p) {
	free(p);
}
*/
import "C"

import (
	"image"
	"io"
	"os"
	"path/filepath"
	"sync"
	"unsafe"
)

// Document represents fitz document.
type Document struct {
	ctx    *C.struct_fz_context
	data   []byte // binds data to the Document lifecycle avoiding premature GC
	doc    *C.struct_fz_document
	mtx    sync.Mutex
	stream *C.fz_stream
}

// New returns new fitz document.
func New(filename string) (f *Document, err error) {
	f = &Document{}

	filename, err = filepath.Abs(filename)
	if err != nil {
		return
	}

	if _, e := os.Stat(filename); e != nil {
		err = ErrNoSuchFile
		return
	}

	f.ctx = (*C.struct_fz_context)(unsafe.Pointer(C.fz_new_context_imp(nil, nil, C.store(MaxStore), C.fz_version)))
	if f.ctx == nil {
		err = ErrCreateContext
		return
	}

	C.fz_register_document_handlers(f.ctx)

	cfilename := C.CString(filename)
	defer C.free(unsafe.Pointer(cfilename))

	f.doc = C.open_document(f.ctx, cfilename)
	if f.doc == nil {
		err = ErrOpenDocument
		return
	}

	ret := C.fz_needs_password(f.ctx, f.doc)
	v := int(ret) != 0
	if v {
		err = ErrNeedsPassword
	}

	return
}

// NewFromMemory returns new fitz document from byte slice.
func NewFromMemory(b []byte) (f *Document, err error) {
	if len(b) == 0 {
		return nil, ErrEmptyBytes
	}
	f = &Document{}

	f.ctx = (*C.struct_fz_context)(unsafe.Pointer(C.fz_new_context_imp(nil, nil, C.store(MaxStore), C.fz_version)))
	if f.ctx == nil {
		err = ErrCreateContext
		return
	}

	C.fz_register_document_handlers(f.ctx)

	f.stream = C.fz_open_memory(f.ctx, (*C.uchar)(&b[0]), C.size_t(len(b)))
	if f.stream == nil {
		err = ErrOpenMemory
		return
	}

	magic := contentType(b)
	if magic == "" {
		err = ErrOpenMemory
		return
	}

	f.data = b

	cmagic := C.CString(magic)
	defer C.free(unsafe.Pointer(cmagic))

	f.doc = C.open_document_with_stream(f.ctx, cmagic, f.stream)
	if f.doc == nil {
		err = ErrOpenDocument
	}

	ret := C.fz_needs_password(f.ctx, f.doc)
	v := int(ret) != 0
	if v {
		err = ErrNeedsPassword
	}

	return
}

// NewFromReader returns new fitz document from io.Reader.
func NewFromReader(r io.Reader) (f *Document, err error) {
	b, e := io.ReadAll(r)
	if e != nil {
		err = e
		return
	}

	f, err = NewFromMemory(b)

	return
}

// NumPage returns total number of pages in document.
func (f *Document) NumPage() int {
	return int(C.fz_count_pages(f.ctx, f.doc))
}

// Image returns image for given page number.
func (f *Document) Image(pageNumber int) (*image.RGBA, error) {
	return f.ImageDPI(pageNumber, 300.0)
}

// ImageDPI returns image for given page number and DPI.
func (f *Document) ImageDPI(pageNumber int, dpi float64) (*image.RGBA, error) {
	return f.imageDPI(pageNumber, dpi, false)
}

// ImageWithAnnotsDPI is like ImageDPI, but also paints every annotation's
// own real appearance (via fz_run_page, which runs both the page's content
// stream and its annotations' appearance streams, instead of
// fz_run_page_contents' content-only render — see run_page's own C-side
// doc comment above). Added for KrankyBearReader: the only way to render
// an annotation whose real appearance isn't a fixed, simple shape a caller
// could reconstruct itself from geometry alone (e.g. a Stamp annotation's
// arbitrary freeform vector art).
func (f *Document) ImageWithAnnotsDPI(pageNumber int, dpi float64) (*image.RGBA, error) {
	return f.imageDPI(pageNumber, dpi, true)
}

func (f *Document) imageDPI(pageNumber int, dpi float64, withAnnots bool) (*image.RGBA, error) {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	if pageNumber >= f.NumPage() {
		return nil, ErrPageMissing
	}

	page := C.load_page(f.ctx, f.doc, C.int(pageNumber))
	if page == nil {
		return nil, ErrLoadPage
	}

	defer C.fz_drop_page(f.ctx, page)

	var bounds C.fz_rect
	bounds = C.fz_bound_page(f.ctx, page)

	var ctm C.fz_matrix
	ctm = C.fz_scale(C.float(dpi/72), C.float(dpi/72))

	var bbox C.fz_irect
	bounds = C.fz_transform_rect(bounds, ctm)
	bbox = C.fz_round_rect(bounds)

	pixmap := C.fz_new_pixmap_with_bbox(f.ctx, C.fz_device_rgb(f.ctx), bbox, nil, 1)
	if pixmap == nil {
		return nil, ErrCreatePixmap
	}

	C.fz_clear_pixmap_with_value(f.ctx, pixmap, C.int(0xff))
	defer C.fz_drop_pixmap(f.ctx, pixmap)

	device := C.fz_new_draw_device(f.ctx, ctm, pixmap)
	C.fz_enable_device_hints(f.ctx, device, C.FZ_NO_CACHE)
	defer C.fz_drop_device(f.ctx, device)

	drawMatrix := C.fz_identity
	var ret C.int
	if withAnnots {
		ret = C.run_page(f.ctx, page, device, drawMatrix, nil)
	} else {
		ret = C.run_page_contents(f.ctx, page, device, drawMatrix, nil)
	}
	if ret == 0 {
		return nil, ErrRunPageContents
	}

	C.fz_close_device(f.ctx, device)

	pixels := C.fz_pixmap_samples(f.ctx, pixmap)
	if pixels == nil {
		return nil, ErrPixmapSamples
	}

	img := image.NewRGBA(image.Rect(int(bbox.x0), int(bbox.y0), int(bbox.x1), int(bbox.y1)))
	copy(img.Pix, C.GoBytes(unsafe.Pointer(pixels), C.int(4*bbox.x1*bbox.y1)))

	return img, nil
}

// ImagePNG returns image for given page number as PNG bytes.
func (f *Document) ImagePNG(pageNumber int, dpi float64) ([]byte, error) {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	if pageNumber >= f.NumPage() {
		return nil, ErrPageMissing
	}

	page := C.load_page(f.ctx, f.doc, C.int(pageNumber))
	if page == nil {
		return nil, ErrLoadPage
	}

	defer C.fz_drop_page(f.ctx, page)

	var bounds C.fz_rect
	bounds = C.fz_bound_page(f.ctx, page)

	var ctm C.fz_matrix
	ctm = C.fz_scale(C.float(dpi/72), C.float(dpi/72))

	var bbox C.fz_irect
	bounds = C.fz_transform_rect(bounds, ctm)
	bbox = C.fz_round_rect(bounds)

	pixmap := C.fz_new_pixmap_with_bbox(f.ctx, C.fz_device_rgb(f.ctx), bbox, nil, 1)
	if pixmap == nil {
		return nil, ErrCreatePixmap
	}

	C.fz_clear_pixmap_with_value(f.ctx, pixmap, C.int(0xff))
	defer C.fz_drop_pixmap(f.ctx, pixmap)

	device := C.fz_new_draw_device(f.ctx, ctm, pixmap)
	C.fz_enable_device_hints(f.ctx, device, C.FZ_NO_CACHE)
	defer C.fz_drop_device(f.ctx, device)

	drawMatrix := C.fz_identity
	ret := C.run_page_contents(f.ctx, page, device, drawMatrix, nil)
	if ret == 0 {
		return nil, ErrRunPageContents
	}

	C.fz_close_device(f.ctx, device)

	buf := C.fz_new_buffer_from_pixmap_as_png(f.ctx, pixmap, C.fz_default_color_params)
	defer C.fz_drop_buffer(f.ctx, buf)

	size := C.fz_buffer_storage(f.ctx, buf, nil)
	str := C.GoStringN(C.fz_string_from_buffer(f.ctx, buf), C.int(size))

	return []byte(str), nil
}

// Links returns slice of links for given page number.
func (f *Document) Links(pageNumber int) ([]Link, error) {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	if pageNumber >= f.NumPage() {
		return nil, ErrPageMissing
	}

	page := C.load_page(f.ctx, f.doc, C.int(pageNumber))
	if page == nil {
		return nil, ErrLoadPage
	}

	defer C.fz_drop_page(f.ctx, page)

	links := C.fz_load_links(f.ctx, page)
	defer C.fz_drop_link(f.ctx, links)

	linkCount := 0
	for currLink := links; currLink != nil; currLink = currLink.next {
		linkCount++
	}

	if linkCount == 0 {
		return nil, nil
	}

	gLinks := make([]Link, linkCount)

	currLink := links
	for i := 0; i < linkCount; i++ {
		gLinks[i] = Link{
			URI: C.GoString(currLink.uri),
		}
		currLink = currLink.next
	}

	return gLinks, nil
}

// Text returns text for given page number.
func (f *Document) Text(pageNumber int) (string, error) {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	if pageNumber >= f.NumPage() {
		return "", ErrPageMissing
	}

	page := C.load_page(f.ctx, f.doc, C.int(pageNumber))
	if page == nil {
		return "", ErrLoadPage
	}

	defer C.fz_drop_page(f.ctx, page)

	var bounds C.fz_rect
	bounds = C.fz_bound_page(f.ctx, page)

	var ctm C.fz_matrix
	ctm = C.fz_scale(C.float(72.0/72), C.float(72.0/72))

	text := C.fz_new_stext_page(f.ctx, bounds)
	defer C.fz_drop_stext_page(f.ctx, text)

	var opts C.fz_stext_options
	opts.flags = 0

	device := C.fz_new_stext_device(f.ctx, text, &opts)
	C.fz_enable_device_hints(f.ctx, device, C.FZ_NO_CACHE)
	defer C.fz_drop_device(f.ctx, device)

	var cookie C.fz_cookie
	ret := C.run_page_contents(f.ctx, page, device, ctm, &cookie)
	if ret == 0 {
		return "", ErrRunPageContents
	}

	C.fz_close_device(f.ctx, device)

	buf := C.fz_new_buffer_from_stext_page(f.ctx, text)
	defer C.fz_drop_buffer(f.ctx, buf)

	str := C.GoString(C.fz_string_from_buffer(f.ctx, buf))

	return str, nil
}

// HTML returns html for given page number.
func (f *Document) HTML(pageNumber int, header bool) (string, error) {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	if pageNumber >= f.NumPage() {
		return "", ErrPageMissing
	}

	page := C.load_page(f.ctx, f.doc, C.int(pageNumber))
	if page == nil {
		return "", ErrLoadPage
	}

	defer C.fz_drop_page(f.ctx, page)

	var bounds C.fz_rect
	bounds = C.fz_bound_page(f.ctx, page)

	var ctm C.fz_matrix
	ctm = C.fz_scale(C.float(72.0/72), C.float(72.0/72))

	text := C.fz_new_stext_page(f.ctx, bounds)
	defer C.fz_drop_stext_page(f.ctx, text)

	var opts C.fz_stext_options
	opts.flags = C.FZ_STEXT_PRESERVE_IMAGES

	device := C.fz_new_stext_device(f.ctx, text, &opts)
	C.fz_enable_device_hints(f.ctx, device, C.FZ_NO_CACHE)
	defer C.fz_drop_device(f.ctx, device)

	var cookie C.fz_cookie
	ret := C.run_page_contents(f.ctx, page, device, ctm, &cookie)
	if ret == 0 {
		return "", ErrRunPageContents
	}

	C.fz_close_device(f.ctx, device)

	buf := C.fz_new_buffer(f.ctx, 1024)
	defer C.fz_drop_buffer(f.ctx, buf)

	out := C.fz_new_output_with_buffer(f.ctx, buf)
	defer C.fz_drop_output(f.ctx, out)

	if header {
		C.fz_print_stext_header_as_html(f.ctx, out)
	}
	C.fz_print_stext_page_as_html(f.ctx, out, text, C.int(pageNumber))
	if header {
		C.fz_print_stext_trailer_as_html(f.ctx, out)
	}

	C.fz_close_output(f.ctx, out)

	str := C.GoString(C.fz_string_from_buffer(f.ctx, buf))

	return str, nil
}

// SVG returns svg document for given page number.
func (f *Document) SVG(pageNumber int) (string, error) {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	if pageNumber >= f.NumPage() {
		return "", ErrPageMissing
	}

	page := C.load_page(f.ctx, f.doc, C.int(pageNumber))
	if page == nil {
		return "", ErrLoadPage
	}

	defer C.fz_drop_page(f.ctx, page)

	var bounds C.fz_rect
	bounds = C.fz_bound_page(f.ctx, page)

	var ctm C.fz_matrix
	ctm = C.fz_scale(C.float(72.0/72), C.float(72.0/72))
	bounds = C.fz_transform_rect(bounds, ctm)

	buf := C.fz_new_buffer(f.ctx, 1024)
	defer C.fz_drop_buffer(f.ctx, buf)

	out := C.fz_new_output_with_buffer(f.ctx, buf)
	defer C.fz_drop_output(f.ctx, out)

	device := C.fz_new_svg_device(f.ctx, out, bounds.x1-bounds.x0, bounds.y1-bounds.y0, C.FZ_SVG_TEXT_AS_PATH, 1)
	C.fz_enable_device_hints(f.ctx, device, C.FZ_NO_CACHE)
	defer C.fz_drop_device(f.ctx, device)

	var cookie C.fz_cookie
	ret := C.run_page_contents(f.ctx, page, device, ctm, &cookie)
	if ret == 0 {
		return "", ErrRunPageContents
	}

	C.fz_close_device(f.ctx, device)
	C.fz_close_output(f.ctx, out)

	str := C.GoString(C.fz_string_from_buffer(f.ctx, buf))

	return str, nil
}

// ToC returns the table of contents (also known as outline).
func (f *Document) ToC() ([]Outline, error) {
	data := make([]Outline, 0)

	outline := C.fz_load_outline(f.ctx, f.doc)
	if outline == nil {
		return nil, ErrLoadOutline
	}
	defer C.fz_drop_outline(f.ctx, outline)

	var walk func(outline *C.fz_outline, level int)

	walk = func(outline *C.fz_outline, level int) {
		for outline != nil {
			res := Outline{}
			res.Level = level
			res.Title = C.GoString(outline.title)
			res.URI = C.GoString(outline.uri)
			res.Page = int(outline.page.page)
			res.Top = float64(outline.y)
			data = append(data, res)

			if outline.down != nil {
				walk(outline.down, level+1)
			}
			outline = outline.next
		}
	}

	walk(outline, 1)
	return data, nil
}

// Metadata returns the map with standard metadata.
func (f *Document) Metadata() map[string]string {
	data := make(map[string]string)

	lookup := func(key string) string {
		ckey := C.CString(key)
		defer C.free(unsafe.Pointer(ckey))

		buf := make([]byte, 256)
		C.fz_lookup_metadata(f.ctx, f.doc, ckey, (*C.char)(unsafe.Pointer(&buf[0])), C.int(len(buf)))

		return string(buf)
	}

	data["format"] = lookup("format")
	data["encryption"] = lookup("encryption")
	data["title"] = lookup("info:Title")
	data["author"] = lookup("info:Author")
	data["subject"] = lookup("info:Subject")
	data["keywords"] = lookup("info:Keywords")
	data["creator"] = lookup("info:Creator")
	data["producer"] = lookup("info:Producer")
	data["creationDate"] = lookup("info:CreationDate")
	data["modDate"] = lookup("info:modDate")

	return data
}

// Bound gives the Bounds of a given Page in the document.
func (f *Document) Bound(pageNumber int) (image.Rectangle, error) {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	if pageNumber >= f.NumPage() {
		return image.Rectangle{}, ErrPageMissing
	}

	page := C.load_page(f.ctx, f.doc, C.int(pageNumber))
	if page == nil {
		return image.Rectangle{}, ErrLoadPage
	}

	defer C.fz_drop_page(f.ctx, page)

	var bounds C.fz_rect
	bounds = C.fz_bound_page(f.ctx, page)

	return image.Rect(int(bounds.x0), int(bounds.y0), int(bounds.x1), int(bounds.y1)), nil
}

// Close closes the underlying fitz document.
func (f *Document) Close() error {
	if f.stream != nil {
		C.fz_drop_stream(f.ctx, f.stream)
	}

	C.fz_drop_document(f.ctx, f.doc)
	C.fz_drop_context(f.ctx)

	f.data = nil

	return nil
}

// TextChar is one character of a page's text with its exact position, as
// returned by TextChars.
type TextChar struct {
	// R is the character; '\n' marks a line or block break (such entries
	// have a zero Quad), so the R values of TextChars concatenate to exactly
	// what Text returns for the same page.
	R rune
	// Quad is the character's corners in page points, origin top-left, y
	// down (MuPDF's page space): upper-left, upper-right, lower-left,
	// lower-right as x,y pairs.
	Quad [8]float32
	// Size is the font size in points.
	Size float32
	// Line is a page-wide line number, shared by every character on a line.
	Line int
}

// TextChars returns every character on page pageNumber (0-based) with its
// exact quad — see TextChar. Added for KrankyBearReader: text-snapped
// highlight drawing and an exact search-match box both need per-character
// positions that Text (no positions) and HTML (line level) don't carry.
func (f *Document) TextChars(pageNumber int) ([]TextChar, error) {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	if pageNumber >= f.NumPage() {
		return nil, ErrPageMissing
	}

	page := C.load_page(f.ctx, f.doc, C.int(pageNumber))
	if page == nil {
		return nil, ErrLoadPage
	}
	defer C.fz_drop_page(f.ctx, page)

	var raw *C.kbr_stext_char
	n := int(C.stext_chars(f.ctx, page, &raw))
	if n < 0 {
		return nil, ErrRunPageContents
	}
	if n == 0 {
		return nil, nil
	}
	defer C.stext_free(raw)

	src := unsafe.Slice(raw, n)
	out := make([]TextChar, n)
	for i := range src {
		out[i].R = rune(src[i].c)
		out[i].Size = float32(src[i].size)
		out[i].Line = int(src[i].line)
		for j := 0; j < 8; j++ {
			out[i].Quad[j] = float32(src[i].quad[j])
		}
	}
	return out, nil
}
