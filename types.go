package main

type CardID struct {
	Set    string
	Number string
}

type CardType int
type CardCategory int
type Attribute int

const (
	Strike Attribute = iota
	Slash
	Ranged
	Special
	Wisdom
)

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

var cardTypeMap map[CardType]string

type CardColor int

const (
	Red CardColor = iota
	Green
	Blue
	Purple
	Black
	Yellow
)

type BaseCard struct {
	ID     CardID
	Types  []CardType
	Colors []CardColor
}

type LeaderCard struct {
	BaseCard
	Power      int
	Life       int
	Attributes []Attribute
}

type CostedCard struct {
	BaseCard
	Cost    int
	Trigger bool
}

type CharacterCard struct {
	CostedCard
	Power      int
	Counter    int
	Attributes []Attribute
}

type EventCard struct {
	CostedCard
}

type StageCard struct {
	CostedCard
}

type CardText struct {
	BaseCard
	Name        string
	Text        string
	TriggerText string
	ImageUrl    string
}

type CardArtVariant struct {
	ReleaseSet string
	ArtType    string
	ImageUrl   string //custom type to verify valid?
}

type AltArts struct { //mention of lang specific?
	BaseCard
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

var langMap map[LanguageID]CardText
