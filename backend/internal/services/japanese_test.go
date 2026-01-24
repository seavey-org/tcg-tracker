package services

import (
	"testing"
)

func TestPokedexNumberExtraction(t *testing.T) {
	tests := []struct {
		name         string
		ocr          string
		wantPokedex  int
		wantCardName string
		wantLanguage string
	}{
		{
			name: "Nidoran with No. 032",
			ocr: `ニドラン♂
HP40
No. 032
つのでつつく`,
			wantPokedex:  32,
			wantCardName: "Nidoran ♂",
			wantLanguage: "Japanese",
		},
		{
			name: "Pikachu with No. 025",
			ocr: `ピカチュウ
HP60
No. 025`,
			wantPokedex:  25,
			wantCardName: "Pikachu",
			wantLanguage: "Japanese",
		},
		{
			name: "Charizard with No. 006",
			ocr: `リザードン
HP120
No. 006`,
			wantPokedex:  6,
			wantCardName: "Charizard",
			wantLanguage: "Japanese",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseOCRText(tt.ocr, "pokemon")

			if result.PokedexNumber != tt.wantPokedex {
				t.Errorf("PokedexNumber = %d, want %d", result.PokedexNumber, tt.wantPokedex)
			}

			if result.CardName != tt.wantCardName {
				t.Errorf("CardName = %q, want %q", result.CardName, tt.wantCardName)
			}

			if result.DetectedLanguage != tt.wantLanguage {
				t.Errorf("DetectedLanguage = %q, want %q", result.DetectedLanguage, tt.wantLanguage)
			}
		})
	}
}

func TestJapaneseTrainerTranslation(t *testing.T) {
	tests := []struct {
		name         string
		ocr          string
		wantCardName string
	}{
		{
			name: "Professor Elm",
			ocr: `TRAINER
ウツギはかせ
あなたの手札をすべて山札にもどし`,
			wantCardName: "Professor Elm",
		},
		{
			name: "Super Rod",
			ocr: `TRAINER
すごいつりざお
コインを投げて`,
			wantCardName: "Super Rod",
		},
		{
			name: "Professor Oak",
			ocr: `TRAINER
オーキドはかせ
手札をすべて捨てる`,
			wantCardName: "Professor Oak",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseOCRText(tt.ocr, "pokemon")

			if result.CardName != tt.wantCardName {
				t.Errorf("CardName = %q, want %q", result.CardName, tt.wantCardName)
			}
		})
	}
}

// TestRealJapaneseOCR tests with actual OCR output from scanned Japanese cards
func TestRealJapaneseOCR(t *testing.T) {
	tests := []struct {
		name         string
		ocr          string
		wantCardName string
		wantLanguage string
		wantPokedex  int
	}{
		{
			name: "Professor Elm - real OCR with TQG garbage",
			ocr: `TQG
ウツギはかせ
あなたの手札をすべて山札にもどし; その
山札をよく切る。 その竜; 山札からカード`,
			wantCardName: "Professor Elm",
			wantLanguage: "Japanese",
		},
		{
			name: "Super Rod - real OCR with @N garbage",
			ocr: `@N町
すごいつりざお
コインを投げて 「おもて」 なら
「進化カード」 を 「うら」 なら`,
			wantCardName: "Super Rod",
			wantLanguage: "Japanese",
		},
		{
			name: "Energy Flow - real OCR",
			ocr: `TBRWG
エネルギーサーキュレート
あなたの場のポケモンについてい
「基本エネルギーカード」 を`,
			wantCardName: "Energy Flow",
			wantLanguage: "Japanese",
		},
		{
			name: "Nidoran - corrupted OCR but has pokedex 092",
			ocr: `こドラン』
HPAO
ovn
とくばりボケモン
身長のm
体重gKg
つのでつつく
30
Na 092`,
			// Note: OCR is too corrupted to translate the name
			// But we should detect Japanese and extract pokedex-like number
			wantLanguage: "Japanese",
			wantPokedex:  92, // OCR read "092" but card is actually No. 032 (Nidoran)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseOCRText(tt.ocr, "pokemon")

			if tt.wantCardName != "" && result.CardName != tt.wantCardName {
				t.Errorf("CardName = %q, want %q", result.CardName, tt.wantCardName)
			}

			if tt.wantLanguage != "" && result.DetectedLanguage != tt.wantLanguage {
				t.Errorf("DetectedLanguage = %q, want %q", result.DetectedLanguage, tt.wantLanguage)
			}

			if tt.wantPokedex != 0 && result.PokedexNumber != tt.wantPokedex {
				t.Errorf("PokedexNumber = %d, want %d", result.PokedexNumber, tt.wantPokedex)
			}
		})
	}
}

func TestTranslateJapaneseName(t *testing.T) {
	tests := []struct {
		japanese string
		english  string
		found    bool
	}{
		{"ピカチュウ", "Pikachu", true},
		{"リザードン", "Charizard", true},
		{"ニドラン♂", "Nidoran ♂", true},
		{"ウツギはかせ", "Professor Elm", true},
		{"すごいつりざお", "Super Rod", true},
		{"Unknown", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.japanese, func(t *testing.T) {
			english, found := TranslateJapaneseName(tt.japanese)
			if found != tt.found {
				t.Errorf("found = %v, want %v", found, tt.found)
			}
			if english != tt.english {
				t.Errorf("english = %q, want %q", english, tt.english)
			}
		})
	}
}

// TestActualOCRFromImages tests with the EXACT OCR output from the 4 test images.
// These are the raw lines captured by running the identifier service on real card images.
func TestActualOCRFromImages(t *testing.T) {
	tests := []struct {
		name         string
		ocr          string
		wantCardName string
		wantLanguage string
		wantPokedex  int
	}{
		{
			name: "Professor Elm - actual OCR from 84f402e1.jpeg",
			ocr: `明心町
ウツギはかせ
|
|
巻フ枚引いて: 手札にする。
|
|
かできない。
|`,
			wantCardName: "Professor Elm",
			wantLanguage: "Japanese",
		},
		{
			name: "Energy Flow - actual OCR from c10c0d05.jpeg",
			ocr: `町心町
エネルギーサーキュレート
|
「基本エネルギーカード」 を
る
好きなだけはがし、 手札に戻して
よい。
|`,
			wantCardName: "Energy Flow",
			wantLanguage: "Japanese",
		},
		{
			name: "Super Rod - actual OCR from 7bfcc556.jpeg with ko instead of go",
			ocr: `|
丁町
すこいつりざお
コインを投げて 「おもて」 なら
「進化カード」 を 「うら」なら
「たねボケモン」 を1枚 あなた
1
に加える。
1`,
			wantCardName: "Super Rod", // Should match via OCR correction こ→ご
			wantLanguage: "Japanese",
		},
		{
			name: "Nidoran male - actual OCR from e8b0852a.jpeg",
			ocr: `{
;
7
|
Zollvp
1
に
ニドラン』
HPAO
o7n
身長05m 体重gKg
とくばりボケモン
つのでつつく
30
コインを投げて 「うら」 なら 相手に
ダメージをあたえることができない。
にげる
耳が大きく、 通くの音
@
耳がば
爾点
'
と角バりなだす。
|
Na 092`,
			wantCardName: "Nidoran ♂", // Should match via corrupted gender symbol handling
			wantLanguage: "Japanese",
			wantPokedex:  92, // OCR corrupted 032 to 092
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseOCRText(tt.ocr, "pokemon")

			if tt.wantCardName != "" && result.CardName != tt.wantCardName {
				t.Errorf("CardName = %q, want %q", result.CardName, tt.wantCardName)
			}

			if tt.wantLanguage != "" && result.DetectedLanguage != tt.wantLanguage {
				t.Errorf("DetectedLanguage = %q, want %q", result.DetectedLanguage, tt.wantLanguage)
			}

			if tt.wantPokedex != 0 && result.PokedexNumber != tt.wantPokedex {
				t.Errorf("PokedexNumber = %d, want %d", result.PokedexNumber, tt.wantPokedex)
			}
		})
	}
}

func TestTranslateJapaneseName_OCRCorrections(t *testing.T) {
	tests := []struct {
		name     string
		japanese string
		english  string
		found    bool
	}{
		// Nidoran with corrupted gender symbols
		{"Nidoran male with 』", "ニドラン』", "Nidoran ♂", true},
		{"Nidoran male with 」", "ニドラン」", "Nidoran ♂", true},
		{"Nidoran male with )", "ニドラン)", "Nidoran ♂", true},
		{"Nidoran female with 『", "ニドラン『", "Nidoran ♀", true},
		{"Nidoran female with 「", "ニドラン「", "Nidoran ♀", true},
		{"Nidoran female with (", "ニドラン(", "Nidoran ♀", true},
		{"Nidoran no gender", "ニドラン", "Nidoran ♂", true},

		// Super Rod with OCR misreads (こ vs ご)
		{"Super Rod with ko instead of go", "すこいつりざお", "Super Rod", true},

		// Dash normalization - OCR often reads ー as various dashes
		{"Rocket's Sneak Attack with hyphen", "ロケット団のおね-さん", "Rocket's Sneak Attack", true},
		{"Rocket's Sneak Attack with en dash", "ロケット団のおね–さん", "Rocket's Sneak Attack", true},
		{"Rocket's Sneak Attack with em dash", "ロケット団のおね—さん", "Rocket's Sneak Attack", true},
		{"Rocket's Sneak Attack with fullwidth hyphen", "ロケット団のおね－さん", "Rocket's Sneak Attack", true},
		{"Rocket's Sneak Attack correct", "ロケット団のおねーさん", "Rocket's Sneak Attack", true},

		// These should still not match
		{"Random Japanese", "ランダム", "", false},
		{"Partial Nidoran", "ニド", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			english, found := TranslateJapaneseName(tt.japanese)
			if found != tt.found {
				t.Errorf("TranslateJapaneseName(%q) found = %v, want %v", tt.japanese, found, tt.found)
			}
			if english != tt.english {
				t.Errorf("TranslateJapaneseName(%q) = %q, want %q", tt.japanese, english, tt.english)
			}
		})
	}
}
