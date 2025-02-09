from selenium import webdriver
from selenium.webdriver.common.by import By
from selenium.webdriver.support.ui import WebDriverWait, Select
from selenium.webdriver.support import expected_conditions as EC
import re
import pprint
import json
BASE_URL = "https://en.onepiece-cardgame.com/cardlist/"

def remove_html_tags(text):
    """Remove all HTML tags from a string."""
    clean_text = re.sub(r'<.*?>', '', text)  # Match anything between < and > and remove
    return clean_text

def scrape_by_color():
    # using js arguments[0].click() to click hidden buttons, selenium gets upset when out of view
    driver = webdriver.Chrome()
    driver.get(BASE_URL)
    all_colors = ['Red', 'Green', 'Blue', 'Purple', 'Black', 'Yellow']
    wait = WebDriverWait(driver, 10)
    all_cards = []
    all_button = driver.find_element(By.XPATH, "//li[text()='ALL']")
    driver.execute_script("arguments[0].click();", all_button)
    for i, color in enumerate(all_colors):
        color_btn = driver.find_element(By.ID, f"color_{color}")
        driver.execute_script("arguments[0].click();", color_btn) # select color
        submit_btn = driver.find_element(By.CSS_SELECTOR, 'div.commonBtn.submitBtn input[type="submit"]')
        submit_btn.click() # search and then wait until loads
        result_col = wait.until(EC.presence_of_element_located((By.CLASS_NAME, "resultCol")))
        color_btn = driver.find_element(By.ID, f"color_{color}") # selenium complains about the first button becoming stale
        driver.execute_script("arguments[0].click();", color_btn) # unselect color
        color_dict = scrape(result_col)
        # easiest way i could think of doing a uniqueness check
        # if the card has a color we already parsed, don't add it again
        # print(color_dict)
        for card in color_dict:
            if not any(color in all_colors[:i] for color in card["colors"]):
                all_cards.append(card)
    return all_cards

def scrape(result_col):
    children = result_col.find_elements(By.XPATH, "./dl")
    result = []
    for child in children:
        child_json = {}
        infoCol = child.find_elements(By.CSS_SELECTOR, ".infoCol")
        name = child.find_elements(By.CSS_SELECTOR, ".cardName")
        cost = child.find_elements(By.CSS_SELECTOR, ".cost")
        attribute = child.find_elements(By.CSS_SELECTOR, ".attribute")
        power = child.find_elements(By.CSS_SELECTOR, ".power")
        counter = child.find_elements(By.CSS_SELECTOR, ".counter")
        color = child.find_elements(By.CSS_SELECTOR, ".color")
        feature = child.find_elements(By.CSS_SELECTOR, ".feature")
        text = child.find_elements(By.CSS_SELECTOR, ".text")
        getInfo = child.find_elements(By.CSS_SELECTOR, ".getInfo")
        lazy = child.find_elements(By.CSS_SELECTOR, ".lazy")
        trigger = child.find_elements(By.CSS_SELECTOR, ".trigger")

        setid, rarity, cardtype = infoCol[0].get_attribute("innerHTML").strip().split('|')
        setid = remove_html_tags(setid).strip()
        rarity = remove_html_tags(rarity).strip()
        cardtype = remove_html_tags(cardtype).strip()
        #print("ID - RARITY - TYPE")
        #print(setid, rarity, cardtype)

        img = lazy[0].get_attribute("data-src")

        child_json['card_id'] = setid
        child_json['rarity'] = rarity
        child_json['card_type'] = cardtype
        child_json['name'] = name[0].get_attribute("innerHTML")
        if cardtype == "LEADER":
            child_json['life'] = cost[0].get_attribute("innerHTML").split('>')[-1]
            if '_' in img:
                child_json['rarity'] = 'ALT L'
        else:
            child_json['cost'] = cost[0].get_attribute("innerHTML").split('>')[-1]
        child_json['attribute'] = remove_html_tags(attribute[0].get_attribute("innerHTML").split()[-1]).split('>')[-1]
        child_json['power'] = power[0].get_attribute("innerHTML").split('>')[-1]
        child_json['counter'] = counter[0].get_attribute("innerHTML").split('>')[-1]
        child_json['colors'] = color[0].get_attribute("innerHTML").split('>')[-1].split('/')
        child_json['types'] = feature[0].get_attribute("innerHTML").split('>')[-1].split('/')
        child_json['text'] = text[0].get_attribute("innerHTML")[15:]
        child_json['art_set'] = getInfo[0].get_attribute("innerHTML")[20:]
        child_json['image_url'] = img
        child_json['trigger'] = '-'
        if trigger:
            child_json['trigger'] = trigger[0].get_attribute("innerHTML")[16:]
        result.append(child_json)
    return result

card_dict = scrape_by_color()
#pprint.pprint(card_dict[:10], indent=2)
with open('card_list.json', 'w') as outfile:
    json.dump(card_dict, outfile, indent=2)
