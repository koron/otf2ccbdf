package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"io"
	"iter"
	"log"
	"log/slog"
	"os"

	"github.com/koron/otf2ccbdf/internal/bitimg"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

func runeIter(face font.Face, filter func(rune) bool) iter.Seq2[rune, fixed.Int26_6] {
	if filter == nil {
		filter = func(rune) bool { return true }
	}
	return func(yield func(rune, fixed.Int26_6) bool) {
		for r := rune(0); r <= 0xffff; r++ {
			adv, ok := face.GlyphAdvance(r)
			if !ok || !filter(r) {
				continue
			}
			if !yield(r, adv) {
				break
			}
		}
	}
}

type BDFConverter struct {
	name string
	face font.Face

	size      int
	halfWidth int
	fullWidth int
	height    int

	ascent  int
	descent int
}

func newBDFConverter(name string, size int) (*BDFConverter, error) {
	// Load a font from a file, determine its family name, and convert it to a font face.
	b, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	fnt, err := opentype.Parse(b)
	if err != nil {
		return nil, err
	}
	familyName, err := fnt.Name(nil, sfnt.NameIDFamily)
	if err != nil {
		slog.Warn("Failed to get family name, so fell back to \"Unknown\"", "err", err)
		familyName = "Unknown"
	}
	face, err := opentype.NewFace(fnt, &opentype.FaceOptions{
		Size:    float64(size),
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, err
	}

	return &BDFConverter{
		name:      familyName,
		face:      face,
		size:      size,
		halfWidth: size / 2,
		fullWidth: size,
		height:    size,
		ascent:    face.Metrics().Ascent.Round(),
		descent:   face.Metrics().Descent.Round(),
	}, nil
}

func (cvt *BDFConverter) Close() error {
	return cvt.face.Close()
}

// Convert converts the font to BDF and write it to the file outName.
func (cvt *BDFConverter) Convert(outName string) error {
	// Open the output file with buffering
	f, err := os.Create(outName)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	defer w.Flush()
	if err := cvt.writeHeader(w); err != nil {
		return err
	}
	return cvt.writeBody(w)
}

// writeHeader Writes the BDF header
func (cvt *BDFConverter) writeHeader(w io.Writer) error {
	// Count the glyphs and calculate their average width
	var (
		glyphCount = 0
		widthSum   = 0
	)
	for _, adv := range runeIter(cvt.face, nil) {
		glyphCount++
		if adv.Round() > cvt.halfWidth {
			widthSum += cvt.fullWidth
		} else {
			widthSum += cvt.halfWidth
		}
	}

	fmt.Fprintln(w, "STARTFONT 2.1")
	fmt.Fprintf(w, "FONT -FreeType-%s-Medium-R-Normal--%d-%d-72-72-C-%d-ISO10646-1\n",
		cvt.name,
		int(((float64(cvt.size)*10*72)/722.7)+0.5), // PixelSize
		cvt.size*10,            // PointSize
		widthSum*10/glyphCount, // AverageWidth
	)
	fmt.Fprintf(w, "SIZE %d 72 72\n", cvt.size)
	fmt.Fprintf(w, "FONTBOUNDINGBOX %d %d 0 %d\n", cvt.fullWidth, cvt.height, -cvt.descent)
	fmt.Fprintf(w, "CHARS %d\n", glyphCount)

	return nil
}

// writeBody writes the BDF body (glyphs)
func (cvt *BDFConverter) writeBody(w io.Writer) error {
	fullImg := bitimg.New(image.Rect(0, 0, cvt.fullWidth, cvt.height))
	halfImg := bitimg.New(image.Rect(0, 0, cvt.halfWidth, cvt.height))
	drawer := &font.Drawer{
		Src:  image.NewUniform(color.White),
		Face: cvt.face,
		Dot:  fixed.Point26_6{},
	}

	for r, adv := range runeIter(cvt.face, nil) {
		var (
			width = cvt.halfWidth
			img   = halfImg
		)
		if adv.Round() > cvt.halfWidth {
			width = cvt.fullWidth
			img = fullImg
		}

		img.Clear()
		drawer.Dst = img
		drawer.Dot = fixed.Point26_6{X: 0, Y: fixed.I(cvt.ascent)}
		drawer.DrawString(fmt.Sprintf("%c", r))

		// Output a character
		fmt.Fprintf(w, "\nSTARTCHAR U+%04X\n", r)
		fmt.Fprintf(w, "ENCODING %d\n", r)
		fmt.Fprintf(w, "DWIDTH %d %d\n", width, 0)
		fmt.Fprintf(w, "BBX %d %d %d %d\n", width, cvt.height, 0, -cvt.descent)
		fmt.Fprintf(w, "BITMAP\n")
		b := img.Bytes()
		xn := img.Xn()
		for len(b) > 0 {
			fmt.Fprintf(w, "%X\n", b[:xn])
			b = b[xn:]
		}
		fmt.Fprintf(w, "ENDCHAR\n")
	}
	return nil
}

// Run converts a OTF/TTF to BDF.
func Run(ctx context.Context, args []string) error {
	var (
		inName  string
		outName string
		size    int
	)

	fs := flag.NewFlagSet("", flag.ExitOnError)
	fs.StringVar(&outName, "out", "", `output name`)
	fs.IntVar(&size, "size", 16, `font size`)
	fs.Parse(args)

	if fs.NArg() == 0 {
		return errors.New("an argument is required: the OTF/TTF file to convert to BDF")
	}
	inName = fs.Arg(0)
	if outName == "" {
		return errors.New("-out must be specified")
	}
	if size%2 == 1 {
		return errors.New("-size must be a multiple of 2")
	}

	cvt, err := newBDFConverter(inName, size)
	if err != nil {
		return err
	}
	defer cvt.Close()
	return cvt.Convert(outName)
}

func main() {
	err := Run(context.Background(), os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
}
