package optcgdb

type CardID struct {
	Set    string
	Number string
}

type CardType int
type CardCategory int
type CardAttribute int

const (
	Strike CardAttribute = iota
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
	CP0
)

var cardTypeMap map[CardType]string = make(map[CardType]string)

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

type Card interface {
	//implement some functions for all card types
}

type LeaderCard struct {
	BaseCard
	Power      int
	Life       int
	Attributes []CardAttribute
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
	Attributes []CardAttribute
}

type EventCard struct {
	CostedCard
}

type StageCard struct {
	CostedCard
}

type CardText struct {
	BaseCard
	Language    LanguageID
	Name        string
	Text        string
	TriggerText string
	ImageUrl    string
}

type CardArtVariant struct {
	ReleaseSet   string
	ArtType      string
	ImageUrl     string //custom type to verify valid?
	TcgPlayerUrl string
}

type AltArts struct {
	CardText
	Variants []CardArtVariant
}

type LanguageID int

const (
	English LanguageID = iota
	Japanese
	Chinese
	French
	Korean
)

var langMap map[LanguageID]CardText = make(map[LanguageID]CardText)
