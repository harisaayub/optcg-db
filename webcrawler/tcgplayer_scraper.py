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

    options = webdriver.ChromeOptions()
    options.add_argument("--disable-blink-features=AutomationControlled")
    options.add_argument("--headless=new")
    options.add_argument("user-agent=Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
    driver = webdriver.Chrome(options=options)
    driver.get(url)
    driver.execute_script("Object.defineProperty(navigator, 'webdriver', {get: () => undefined})")
    wait = WebDriverWait(driver, 10)
    wait.until(EC.presence_of_element_located((By.TAG_NAME, "script")))
    # click into more info and pull last sold data
    print(driver.page_source)
    return Prices([], [])


if __name__ == "__main__":
    print(getPrices("https://www.tcgplayer.com/product/596914/one-piece-card-game-emperors-in-the-new-world-rob-lucci-sp?Language=English"))
