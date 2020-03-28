package renderer

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/fmpwizard/go-quilljs-delta/delta"
)

type Palette struct {
	Black         string
	Red           string
	Green         string
	Yellow        string
	Blue          string
	Magenta       string
	Cyan          string
	White         string
	BrightBlack   string
	BrightRed     string
	BrightGreen   string
	BrightYellow  string
	BrightBlue    string
	BrightMagenta string
	BrightCyan    string
	BrightWhite   string
}

func DefaultPalette() Palette {
	return Palette{
		Black:         "#000000",
		Red:           "#cd0000",
		Green:         "#00cd00",
		Yellow:        "#cdcd00",
		Blue:          "#0000ee",
		Magenta:       "#cd00cd",
		Cyan:          "#00cdcd",
		White:         "#e5e5e5",
		BrightBlack:   "#7f7f7f",
		BrightRed:     "#ff0000",
		BrightGreen:   "#00ff00",
		BrightYellow:  "#ffff00",
		BrightBlue:    "#5c5cff",
		BrightMagenta: "#ff00ff",
		BrightCyan:    "#00ffff",
		BrightWhite:   "#ffffff",
	}
}

var cube6Values = []string{"00", "5f", "87", "af", "d7", "ff"}

func (p Palette) Color(i int) string {
	switch i {
	case 0:
		return p.Black
	case 1:
		return p.Red
	case 2:
		return p.Green
	case 3:
		return p.Yellow
	case 4:
		return p.Blue
	case 5:
		return p.Magenta
	case 6:
		return p.Cyan
	case 7:
		return p.White
	}
	return ""
}

func (p Palette) HighIntensityColor(i int) string {
	switch i {
	case 0:
		return p.BrightBlack
	case 1:
		return p.BrightRed
	case 2:
		return p.BrightGreen
	case 3:
		return p.BrightYellow
	case 4:
		return p.BrightBlue
	case 5:
		return p.BrightMagenta
	case 6:
		return p.BrightCyan
	case 7:
		return p.BrightWhite
	}
	return ""
}

func StyleEraser() map[string]interface{} {
	return map[string]interface{}{
		"bold":      nil,
		"color":     nil,
		"italic":    nil,
		"underline": nil,
	}
}

type Renderer struct {
	c            chan<- delta.Delta
	left         []byte
	currentStyle map[string]interface{}
	palette      Palette
}

func NewRenderer(c chan<- delta.Delta) *Renderer {
	return NewRendererWithPalette(c, DefaultPalette())
}

func NewRendererWithPalette(c chan<- delta.Delta, palette Palette) *Renderer {
	return &Renderer{
		c:       c,
		palette: palette,
	}
}

func (r *Renderer) Write(p []byte) (int, error) {
	r.left = append(r.left, p...)
	// TODO: non-UTF8 support later
	for len(r.left) > 0 {
		rs := make([]rune, 0)
		for len(r.left) > 0 {
			ch, size, err := r.peekRune(0)
			if err != nil {
				return 0, err
			}
			if size == 0 {
				// We haven't read a full rune yet.
				return len(p), nil
			}
			if ch == 0x1b {
				if len(rs) > 0 {
					r.c <- *delta.New(nil).Insert(string(rs), r.currentStyle)
					rs = make([]rune, 0)
				}
				ch2, size2, err := r.peekRune(size)
				if err != nil {
					return 0, err
				}
				if size2 == 0 {
					return len(p), nil
				}
				switch ch2 {
				case '[': // CSI: vt100 4.3.3
					data, peekBytes, err := r.peekTill(size+size2, 0x40, 0x7F)
					if err != nil {
						return 0, err
					}
					if peekBytes == 0 {
						return len(p), nil
					}
					r.handleCSI(data)
					r.left = r.left[size+size2+peekBytes:]
				case 'U': // Custom extension by us, allows inserting data URI images
					data, peekBytes, err := r.peekTill(size+size2, ';', ';'+1)
					if err != nil {
						return 0, err
					}
					if peekBytes == 0 {
						return len(p), nil
					}
					length, err := strconv.Atoi(string(data[:len(data)-1]))
					if err != nil {
						return 0, err
					}
					imageStart := size + size2 + peekBytes
					if len(r.left) < imageStart+length {
						// Not enough bytes
						return len(p), nil
					}
					imageData := r.left[imageStart : imageStart+length]
					if !utf8.Valid(imageData) {
						return 0, fmt.Errorf("Image data is not in UTF-8 format!")
					}
					r.c <- *delta.New(nil).InsertEmbed(delta.Embed{
						Key:   "image",
						Value: string(imageData),
					}, r.currentStyle)
					r.left = r.left[imageStart+length:]
				default:
					r.left = r.left[size+size2:]
				}
			} else {
				rs = append(rs, ch)
				r.left = r.left[size:]
			}
		}
		if len(rs) > 0 {
			r.c <- *delta.New(nil).Insert(string(rs), r.currentStyle)
		}
	}
	return len(p), nil
}

func (r *Renderer) peekTill(start int, endCharStart rune, endCharExclusiveEnd rune) ([]rune, int, error) {
	rs := make([]rune, 0)
	peekBytes := 0
	for start+peekBytes < len(r.left) {
		ch, size, err := r.peekRune(start + peekBytes)
		if err != nil {
			return nil, 0, err
		}
		if size == 0 {
			return nil, 0, nil
		}
		rs = append(rs, ch)
		peekBytes += size
		if ch >= endCharStart && ch < endCharExclusiveEnd {
			return rs, peekBytes, nil
		}
	}
	return nil, 0, nil
}

func (r *Renderer) peekRune(start int) (rune, int, error) {
	ch, size := utf8.DecodeRune(r.left[start:])
	if ch == utf8.RuneError {
		if len(r.left[start:]) < 4 {
			return 0, 0, nil
		} else {
			return 0, 0, fmt.Errorf("Invalid UTF-8 sequence!")
		}
	}
	return ch, size, nil
}

func (r *Renderer) handleCSI(data []rune) error {
	command := data[len(data)-1]
	paramRunes := make([]rune, 0)
	for _, v := range data[:len(data)-1] {
		if v >= 0x30 && v < 0x40 {
			paramRunes = append(paramRunes, v)
		} else {
			break
		}
	}
	params := make([]int, 0)
	for _, v := range strings.Split(string(paramRunes), ";") {
		vi, err := strconv.Atoi(v)
		if err != nil {
			return err
		}
		params = append(params, vi)
	}
	switch command {
	case 'm':
		if len(params) == 0 {
			params = []int{0}
		}
		i := 0
		for i < len(params) {
			v := params[i]
			i = i + 1
			previousIsOne := false
			if v == 1 {
				if i < len(params) && params[i] >= 30 && params[i] <= 37 {
					previousIsOne = true
					v = params[i]
					i = i + 1
				} else {
					r.setStyle("bold", true)
					continue
				}
			}
			switch v {
			case 0:
				r.clearStyle()
			case 3:
				r.setStyle("italic", true)
			case 4:
				r.setStyle("underline", true)
			case 21:
				r.setStyle("bold", nil)
			case 22:
				r.setStyle("bold", nil)
				r.setStyle("color", nil)
			case 23:
				r.setStyle("italic", nil)
			case 24:
				r.setStyle("underline", nil)
			case 30:
				fallthrough
			case 31:
				fallthrough
			case 32:
				fallthrough
			case 33:
				fallthrough
			case 34:
				fallthrough
			case 35:
				fallthrough
			case 36:
				fallthrough
			case 37:
				if previousIsOne {
					r.setStyle("color", r.palette.HighIntensityColor(v-30))
				} else {
					r.setStyle("color", r.palette.Color(v-30))
				}
			case 38:
				if i >= len(params) {
					return fmt.Errorf("Not enough params to set-color(38)!")
				}
				mode := params[i]
				i = i + 1
				switch mode {
				case 5:
					if i >= len(params) {
						return fmt.Errorf("Not enough params to set-color(38)!")
					}
					colorInt := params[i]
					i = i + 1
					var actualColor string
					if colorInt < 8 {
						actualColor = r.palette.Color(colorInt)
					} else if colorInt < 16 {
						actualColor = r.palette.HighIntensityColor(colorInt - 8)
					} else if colorInt < 232 {
						b := (colorInt - 16) % 6
						g := ((colorInt - 16) / 6) % 6
						r := (((colorInt - 16) / 6) / 6) % 6
						actualColor = fmt.Sprintf("#%s%s%s", cube6Values[r], cube6Values[g], cube6Values[b])
					} else {
						actualColor = fmt.Sprintf("#%06x", 0x80808+(colorInt-232)*0xa0a0a)
					}
					r.setStyle("color", actualColor)
				case 2:
					if i+3 > len(params) {
						return fmt.Errorf("Not enough params to set-color(38)!")
					}
					red := params[i] & 0xFF
					g := params[i+1] & 0xFF
					b := params[i+2] & 0xFF
					i = i + 3
					actualColor := fmt.Sprintf("#%06x", (red<<16)|(g<<8)|b)
					r.setStyle("color", actualColor)
				default:
					return fmt.Errorf("Unsupported set-color(38) mode: %d!", mode)
				}
			default:
				return fmt.Errorf("Unsupported SGR parameter: %d", v)
			}
		}
	default:
		return fmt.Errorf("Invalid CSI command: %c", command)
	}
	return nil
}

func (r *Renderer) clearStyle() {
	r.currentStyle = nil
}

func (r *Renderer) setStyle(name string, value interface{}) {
	newStyle := make(map[string]interface{})
	for k, v := range r.currentStyle {
		newStyle[k] = v
	}
	if value != nil && value != "" {
		newStyle[name] = value
	} else {
		delete(newStyle, name)
	}
	r.currentStyle = newStyle
}
