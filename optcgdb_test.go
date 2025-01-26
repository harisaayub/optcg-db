package optcgdb

import (
	"fmt"
	"testing"
)

func Test(t *testing.T) {
	var rl_base BaseCard = BaseCard{CardID{"OP05", "093"}, []CardType{CP0}, []CardColor{Black}}
	var rl CharacterCard = CharacterCard{CostedCard{rl_base, 4, false}, 6000, 0, []CardAttribute{Strike}}
	var rl_text CardText = CardText{rl_base, English, "Rob Lucci",
		"[On Play] You may place 3 cards from your trash at the bottom of your deck in any order: K.O. up to 1 of your opponent's Characters with a cost of 2 or less and up to 1 of your opponent's Characters with a cost of 1 or less.",
		"",
		"https://en.onepiece-cardgame.com/images/cardlist/card/OP05-093.png"}
	print(rl_text.BaseCard.ID.Number + "\n")
	fmt.Println(rl.Colors)
}
