package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"log/slog"
	"os"

	"github.com/koron/otf2ccbdf/internal/bitimg"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

func loadFont(name string) (*opentype.Font, error) {
	b, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	return opentype.Parse(b)
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

	return toBDF(outName, inName, size)
}

func toBDF(outFileName string, fontFileName string, fontSize int) error {
	fnt, err := loadFont(fontFileName)
	if err != nil {
		return err
	}

	familyName, err := fnt.Name(nil, sfnt.NameIDFamily)
	if err != nil {
		slog.Warn("Failed to get family name, so fell back to \"Unknown\"", "err", err)
		familyName = "Unknown"
	}

	face, err := opentype.NewFace(fnt, &opentype.FaceOptions{
		Size:    float64(fontSize),
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return err
	}
	defer face.Close()

	halfWidth := fontSize / 2

	fullImg := bitimg.New(image.Rect(0, 0, fontSize, fontSize))
	halfImg := bitimg.New(image.Rect(0, 0, halfWidth, fontSize))

	drawer := &font.Drawer{
		Src:  image.NewUniform(color.White),
		Face: face,
		Dot:  fixed.Point26_6{},
	}
	ascent := face.Metrics().Ascent
	descent := face.Metrics().Descent

	f, err := os.Create(outFileName)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	defer w.Flush()

	fmt.Fprintln(w, "STARTFONT 2.1")
	fmt.Fprintf(w, "FONT -FreeType-%s-Medium-R-Normal--%d-%d-72-72-C-%d-ISO10646-1\n",
		familyName,
		int(((float64(fontSize)*10*72)/722.7)+0.5), // PixelSize
		fontSize*10, // PointSize
		fontSize*10, // AverageWidth
	)
	fmt.Fprintf(w, "SIZE %d 72 72\n", fontSize)
	fmt.Fprintf(w, "FONTBOUNDINGBOX %d %d 0 %d\n", fontSize, fontSize, -descent.Round())
	//fmt.Fprintf(w, "CHARS %d\n", fnt.NumGlyphs())

	count := 0
	for r := rune(0); r <= 0xffff; r++ {
		// Skip unavailable glyphs
		adv, ok := face.GlyphAdvance(r)
		if !ok {
			continue
		}

		var (
			width = halfWidth
			img   = halfImg
		)
		if adv.Round() > halfWidth {
			width = fontSize
			img = fullImg
		}

		img.Clear()
		drawer.Dst = img
		drawer.Dot = fixed.Point26_6{X: 0, Y: ascent}
		drawer.DrawString(fmt.Sprintf("%c", r))

		// Output a character
		fmt.Fprintf(w, "\nSTARTCHAR U+%04X\n", r)
		fmt.Fprintf(w, "ENCODING %d\n", r)
		fmt.Fprintf(w, "DWIDTH %d %d\n", width, 0)
		fmt.Fprintf(w, "BBX %d %d %d %d\n", width, fontSize, 0, -descent.Round())
		fmt.Fprintf(w, "BITMAP\n")
		b := img.Bytes()
		xn := img.Xn()
		for len(b) > 0 {
			fmt.Fprintf(w, "%X\n", b[:xn])
			b = b[xn:]
		}
		fmt.Fprintf(w, "ENDCHAR\n")
		count++
	}

	return nil
}

func main() {
	err := Run(context.Background(), os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
}
