from selenium import webdriver
from selenium.webdriver.common.by import By
from selenium.webdriver.support.ui import WebDriverWait
from selenium.webdriver.support import expected_conditions as EC

class Prices:
    def __init__(self, last_sold, lowest_verified):
        self.last_sold = last_sold
        self.lowest_verified = lowest_verified
    def __str__(self):
        return f"Last Sold: {self.last_sold}\nLowest Verified: {self.lowest_verified}"

def getPrices(url: str) -> Prices:
    driver = webdriver.Chrome()
    driver.get(url)
    element = driver.find_element(By.CLASS_NAME, "spotlight__price")
    # click into more info and pull last sold data
    return Prices([], [])


if __name__ == "__main__":
    print(getPrices("https://www.tcgplayer.com/product/596914/one-piece-card-game-emperors-in-the-new-world-rob-lucci-sp?Language=English"))
