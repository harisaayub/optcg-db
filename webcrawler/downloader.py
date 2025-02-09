from selenium import webdriver
from selenium.webdriver.common.by import By
from selenium.webdriver.support.ui import WebDriverWait
from selenium.webdriver.support import expected_conditions as EC
import re
import pprint
import json

def remove_html_tags(text):
    """Remove all HTML tags from a string."""
    clean_text = re.sub(r'<.*?>', '', text)  # Match anything between < and > and remove
    return clean_text

def drive(url):
    driver = webdriver.Chrome()
    driver.get(url)
    result_col = driver.find_element(By.CLASS_NAME, "resultCol")
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
        child_json['name'] = name[0].get_attribute("innerHTML").replace('&amp;', '&')
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
        child_json['text'] = text[0].get_attribute("innerHTML")[15:].replace('<br>', '\n').replace('\u2013', '-').replace('\u2212', '-')
        child_json['art_set'] = getInfo[0].get_attribute("innerHTML").split('[')[-1][:-1]
        child_json['image_url'] = img
        child_json['trigger'] = '-'
        if trigger:
            child_json['trigger'] = trigger[0].get_attribute("innerHTML")[16:].replace('<br>', '\n').replace('\u2013', '-').replace('\u2212', '-')
        result.append(child_json)
    return result

card_dict = drive("https://en.onepiece-cardgame.com/cardlist/")
#pprint.pprint(card_dict[:10], indent=2)
with open('card_list.json', 'w') as outfile:
    json.dump(card_dict, outfile, indent=2)
