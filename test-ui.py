from playwright.sync_api import sync_playwright
import time

with sync_playwright() as p:
    browser = p.chromium.launch(headless=True)
    page = browser.new_page(viewport={"width": 1440, "height": 900})

    # Login page
    page.goto("http://localhost:8080/ui/login")
    page.wait_for_load_state("networkidle")
    time.sleep(1)
    page.screenshot(path="/tmp/screenshot-login.png", full_page=True)
    print("Login page screenshot saved")

    # Fill token and login
    page.fill('input[type="password"]', "admin-token-12345")
    page.fill('input[placeholder="t-default"]', "t-default")
    page.click('button[type="submit"]')
    page.wait_for_load_state("networkidle")
    time.sleep(2)

    # Dashboard
    page.screenshot(path="/tmp/screenshot-dashboard.png", full_page=True)
    print("Dashboard screenshot saved")

    # Navigate to Keys
    page.goto("http://localhost:8080/ui/keys")
    page.wait_for_load_state("networkidle")
    time.sleep(1)
    page.screenshot(path="/tmp/screenshot-keys.png", full_page=True)
    print("Keys page screenshot saved")

    # Create a key
    page.click('button:has-text("Create Key")')
    time.sleep(0.5)
    page.fill('input[placeholder="my-app-key"]', "frontend-test-key")
    page.click('button:has-text("Create")')
    page.wait_for_load_state("networkidle")
    time.sleep(1)
    page.screenshot(path="/tmp/screenshot-keys-created.png", full_page=True)
    print("Keys created screenshot saved")

    # Navigate to Crypto sandbox
    page.goto("http://localhost:8080/ui/crypto")
    page.wait_for_load_state("networkidle")
    time.sleep(1)
    page.screenshot(path="/tmp/screenshot-crypto.png", full_page=True)
    print("Crypto sandbox screenshot saved")

    # Navigate to Policy
    page.goto("http://localhost:8080/ui/policy")
    page.wait_for_load_state("networkidle")
    time.sleep(1)
    page.screenshot(path="/tmp/screenshot-policy.png", full_page=True)
    print("Policy page screenshot saved")

    # Navigate to Data Keys
    page.goto("http://localhost:8080/ui/data-keys")
    page.wait_for_load_state("networkidle")
    time.sleep(1)
    page.screenshot(path="/tmp/screenshot-datakeys.png", full_page=True)
    print("Data Keys page screenshot saved")

    # Navigate to Audit
    page.goto("http://localhost:8080/ui/audit")
    page.wait_for_load_state("networkidle")
    time.sleep(1)
    page.screenshot(path="/tmp/screenshot-audit.png", full_page=True)
    print("Audit page screenshot saved")

    browser.close()
    print("All screenshots saved to /tmp/")
