package ws

import (
	"bytes"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestTextReaderNext(t *testing.T) {
	r := strings.NewReader("  \t\t \t\n \n\n\t\n\t \n\n\n")
	l := NewTextReader(r)
	var tokens []SpaceToken
	for {
		token, err := l.Next()
		if err != nil {
			t.Fatal(err)
		}
		tokens = append(tokens, token)
		if token == EOF {
			break
		}
	}
	want := []SpaceToken{Space, Space, Tab, Tab, Space, Tab, LF,
		Space, LF, LF, Tab, LF, Tab, Space, LF, LF, LF, EOF}
	if !reflect.DeepEqual(tokens, want) {
		t.Errorf("got %s, want %s", tokensString(tokens), tokensString(want))
	}
}

func TestBitReaderNext(t *testing.T) {
	r := bytes.NewReader([]byte{0x29, 0x6F, 0xB9, 0xF8}) // 00101001 01101111 10111001 11111(000)
	l := NewBitReader(r)
	var tokens []SpaceToken
	for {
		token, err := l.Next()
		if err != nil {
			t.Fatal(err)
		}
		tokens = append(tokens, token)
		if token == EOF {
			break
		}
	}
	want := []SpaceToken{Space, Space, Tab, Tab, Space, Tab, LF,
		Space, LF, LF, Tab, LF, Tab, Space, LF, LF, LF, Space, Space, Space, EOF}
	if !reflect.DeepEqual(tokens, want) {
		t.Errorf("got %s, want %s", tokensString(tokens), tokensString(want))
	}
}

func tokensString(tokens []SpaceToken) string {
	var str strings.Builder
	str.WriteRune('[')
	for i, token := range tokens {
		if i != 0 {
			str.WriteRune(' ')
		}
		switch token {
		case EOF:
			str.WriteString("EOF")
		case Space:
			str.WriteString("Space")
		case Tab:
			str.WriteString("Tab")
		case LF:
			str.WriteString("LF")
		default:
			str.WriteString(strconv.Itoa(int(token)))
		}
	}
	str.WriteRune(']')
	return str.String()
}
