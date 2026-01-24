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
			name: "Energy Circulator - real OCR",
			ocr: `TBRWG
エネルギーサーキュレート
あなたの場のポケモンについてい
「基本エネルギーカード」 を`,
			wantCardName: "Energy Circulator",
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
