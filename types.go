package main

type CardID struct {
	Set    string
	Number string
}

type CardType int
type CardCategory int

const (
	Leader CardCategory = iota
	Character
	Event
	Stage
)

const (
	StrawHatCrew CardType = iota
	Egghead
)

type CardColor int

const (
	Red CardColor = iota
	Green
	Blue
	Purple
	Black
	Yellow
)

type CardInfo struct {
	ID       CardID
	Types    []CardType
	Category CardCategory
}

type LeaderCardData struct {
	ID        CardID
	Power     int
	Life      int
	Attribute int
	Colors    []CardColor
}

type CharacterCardData struct {
	ID        CardID
	Power     int
	Cost      int
	Attribute int
	Color     []CardColor
	//trigger?
}

type EventCardData struct {
	ID    CardID
	Cost  int
	Color []CardColor
	//trigger?
}

type StageCardData struct {
	ID    CardID
	Cost  int
	Color []CardColor
	//trigger?
}

type LanguageSpecificCardData struct {
	ID   CardID
	Name string
	Text string
	//TriggerText string
	imageUrl string //custom type to verify valid?
}

type CardArtVariant struct {
	ReleaseSet string
	ArtType    string
	imageUrl   string //custom type to verify valid?
}

type AltArtData struct { //mention of lang specific?
	ID       CardID
	variants []CardArtVariant
}

type LanguageID int

const (
	English LanguageID = iota
	Japanese
	Chinese
	French
	Korean
)

var langMap map[LanguageID]LanguageSpecificCardData
