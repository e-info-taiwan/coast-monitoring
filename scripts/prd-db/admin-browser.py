"""Exercise the admin UI in Chrome against the CI server and its disposable PostgreSQL database."""
import json
import os
import secrets
import subprocess
import time
import urllib.error
import urllib.request

from playwright.sync_api import expect, sync_playwright

BASE = "http://127.0.0.1:8080"
DB = os.environ["REEF_DATA_TEST_DATABASE_URL"]
EMAIL = "browser-test@example.invalid"
PASSWORD = secrets.token_hex(24)


def seed():
    # This fixture belongs exclusively to the CI PostgreSQL service.
    if os.environ.get("GITHUB_ACTIONS") != "true" or DB != "postgres://postgres:test@localhost:5432/coast_test?sslmode=disable":
        raise RuntimeError("A disposable CI database is required")
    subprocess.run(["psql", DB, "-Xq", "-v", "ON_ERROR_STOP=1", "-v", "password=" + PASSWORD], input="""
        INSERT INTO users(email,name,role,password_hash) VALUES ('browser-test@example.invalid','Browser Test','admin',crypt(:'password',gen_salt('bf')));
        INSERT INTO site(name_zh,name_en) VALUES('瀏覽器驗收站','BrowserFixture') RETURNING id AS site_id \\gset
        INSERT INTO survey(site_id,start_date,end_date) VALUES(:site_id,'2026-11-06','2026-11-06') RETURNING id AS survey_id \\gset
        INSERT INTO event(survey_id,event_id,survey_date,event_time,depth_m,publication_status)
        VALUES(:survey_id,'browser-member','2026-11-06','09:00',5,'published'),(:survey_id,'browser-other','2026-11-06','14:00',5,'published');
        INSERT INTO transect(event_id,method) VALUES('browser-member','line') RETURNING id AS transect_id \\gset
        INSERT INTO transect(event_id,method) VALUES('browser-member','belt_invert'),('browser-other','line');
        INSERT INTO diver(name_zh,reef_check_code,is_active) VALUES('驗收潛水員','RC-DIVER-TEST',false) RETURNING id AS diver_id \\gset
        INSERT INTO transect_participant(transect_id,diver_id,role) VALUES(:transect_id,:diver_id,'member'),(:transect_id,:diver_id,'team_leader');
    """, text=True, check=True)


def verify(code, expected_status):
    request = urllib.request.Request(BASE + "/api/public/reef-check/activities/verify", data=json.dumps({"code": code}).encode(), headers={"Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(request, timeout=10) as response:
            status = response.status
    except urllib.error.HTTPError as error:
        status = error.code
    assert status == expected_status, (status, expected_status)


def test_ui():
    with sync_playwright() as playwright:
        browser = playwright.chromium.launch(channel="chrome", args=["--no-sandbox"])
        context = browser.new_context(permissions=["clipboard-read", "clipboard-write"], viewport={"width": 1280, "height": 800})
        page = context.new_page()
        page.on("dialog", lambda dialog: dialog.accept())
        page.goto(BASE + "/admin/")
        page.locator("#dev-email").fill(EMAIL)
        page.locator("#dev-password").fill(PASSWORD)
        page.get_by_role("button", name="Sign in", exact=True).click()
        filters = page.locator("#rd-filters")
        filters.get_by_label("潛水員").fill("RC-DIVER-TEST")
        filters.get_by_text("篩選", exact=True).click()
        expect(page.get_by_role("button", name="查看 browser-member", exact=True)).to_have_count(1)
        expect(page.get_by_role("button", name="查看 browser-other", exact=True)).to_have_count(0)
        filters.get_by_label("調查方法").select_option("belt_invert")
        filters.get_by_text("篩選", exact=True).click()
        expect(page.get_by_text("沒有符合條件的場次。", exact=True)).to_be_visible()
        filters.get_by_role("button", name="清除").click()
        expect(page.get_by_role("button", name="查看 browser-other", exact=True)).to_be_visible()

        page.get_by_role("link", name="管理活動與志工填報代碼").click()
        page.locator('[data-pc="new"]').first.click()
        form = page.locator("#pc-edit-form")
        form.locator('[name="title"]').fill("瀏覽器驗收活動")
        form.locator('[name="start_at"]').fill("2026-11-06T09:00")
        form.locator('[name="end_at"]').fill("2026-11-06T12:00")
        enabled = form.locator('[name="entry_enabled"]')
        enabled.select_option("true")
        save = form.locator('[type="submit"]')
        save.click()
        expect(form.get_by_role("alert")).to_contain_text("勾選")
        form.get_by_label("瀏覽器驗收站").check()
        save.click()
        output = page.locator(".pc-entry-code output")
        expect(output).to_be_in_viewport()
        first = output.inner_text()
        assert first.startswith("RC-") and len(first) == 35
        page.get_by_role("button", name="複製代碼").click()
        assert page.evaluate("navigator.clipboard.readText()") == first
        verify(first, 200)
        enabled.select_option("false")
        save.click()
        expect(output).to_have_count(0)
        enabled.select_option("true")
        save.click()
        expect(output).to_be_in_viewport()
        second = output.inner_text()
        assert second != first
        verify(first, 400)
        verify(second, 200)
        page.reload()
        page.get_by_text("瀏覽器驗收活動", exact=True).first.wait_for()
        page.locator("tr").filter(has_text="瀏覽器驗收活動").get_by_role("button").first.click()
        page.get_by_role("button", name="重發代碼").click()
        expect(output).to_be_in_viewport()
        verify(output.inner_text(), 200)
        verify(second, 400)
        print("Chrome passed: diver search, method intersection, clear, required site, create, copy, volunteer verification, re-enable, reopen and reissue.")
        browser.close()


if __name__ == "__main__":
    seed()
    with subprocess.Popen(["go", "run", "./cmd/server"], env={**os.environ, "DATABASE_URL": DB, "SESSION_SECRET": secrets.token_hex(32), "ADMIN_ALLOWED_ORIGINS": BASE, "SECURE_COOKIES": "false", "ENABLE_CWA_CRON": "false"}) as server:
        try:
            for _ in range(120):
                try:
                    urllib.request.urlopen(BASE + "/api/session", timeout=1).close()
                    break
                except urllib.error.URLError:
                    time.sleep(0.5)
            test_ui()
        finally:
            server.terminate()
