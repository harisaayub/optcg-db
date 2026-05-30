"""
Scrapes https://en.onepiece-cardgame.com/news/blockicon-card.html for card IDs
that are permitted in Standard Regulation despite being from rotated sets.
Writes ../permitted_cards.json as a sorted list of card ID strings.
"""
import re
import json
import requests
from bs4 import BeautifulSoup

URL = "https://en.onepiece-cardgame.com/news/blockicon-card.html"
CARD_ID_RE = re.compile(r'\b([A-Z]{2}\d{2}-\d{3})\b')

def scrape_permitted_cards() -> list[str]:
    resp = requests.get(URL, timeout=15)
    resp.raise_for_status()
    soup = BeautifulSoup(resp.text, "html.parser")
    ids = set()
    for tag in soup.find_all(string=CARD_ID_RE):
        for match in CARD_ID_RE.finditer(tag):
            ids.add(match.group(1))
    # Also catch IDs inside href attributes like ?card_id=OP01-016
    for a in soup.find_all("a", href=True):
        for match in CARD_ID_RE.finditer(a["href"]):
            ids.add(match.group(1))
    return sorted(ids)

if __name__ == "__main__":
    cards = scrape_permitted_cards()
    print(f"Found {len(cards)} permitted cards")
    for c in cards:
        print(" ", c)
    with open("../permitted_cards.json", "w") as f:
        json.dump(cards, f, indent=2)
    print("Written to ../permitted_cards.json")
