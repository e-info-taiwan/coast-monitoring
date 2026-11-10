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
        INSERT INTO substrate_point(transect_id,segment,position_m,substrate_layer,substrate_code) VALUES
          (:transect_id,3,61,'surface','OT'),(:transect_id,4,94.5,'surface','OT'),(:transect_id,1,10,'surface','NA'),
          (:transect_id,1,0,'surface','RC'),(:transect_id,1,0,'down','HC');
        INSERT INTO substrate_point(transect_id,segment,position_m,substrate_layer,substrate_code) VALUES
          (:transect_id,2,25,'surface','HC-a'),(:transect_id,2,25.5,'surface','HC'),(:transect_id,3,50,'surface','HC-b');
        INSERT INTO substrate_bleaching(transect_id,segment,hc_bleached_count,sc_bleached_count) VALUES
          (:transect_id,2,1,0),(:transect_id,3,0,0),(:transect_id,4,NULL,0);
        UPDATE event SET submission_snapshot='{"format":"reefcheck-web-v4","bleaching_mode":"point_marks"}'
          WHERE event_id='browser-other';
        INSERT INTO substrate_point(transect_id,segment,position_m,substrate_code,is_bleached)
          SELECT t.id,v.segment,v.position_m,v.code,v.mark FROM transect t CROSS JOIN
          (VALUES (1,0,'HC-a',true),(1,0.5,'HC',false),(1,1,'SC',false),(2,25,'HC',NULL),(3,50,'NA',NULL))
          AS v(segment,position_m,code,mark) WHERE t.event_id='browser-other';
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


def check_substrate_board(page):
    page.get_by_role("button", name="查看 browser-member", exact=True).click()
    page.get_by_role("button", name="底質 Line", exact=True).click()
    surface = page.get_by_role("table", name="表層逐點底質")
    down = page.get_by_role("table", name="下層逐點底質")
    rows = surface.locator("tbody tr")
    expect(rows).to_have_count(20)
    for row_index, row in enumerate(rows.all()):
        positions = row.locator(".rd-distance").all_text_contents()
        assert [float(p) for p in positions] == [
            start + offset + row_index / 2 for start in (0, 25, 50, 75) for offset in (0, 10)
        ]
    expect(rows.first.locator("td").nth(1)).to_have_text("6")
    expect(rows.first.locator("td").nth(3)).to_have_text("NA")
    expect(rows.first.locator("td").nth(7)).to_have_text("—")
    expect(surface.get_by_label("表層第 3 段 61 m 底質：OT · 其他", exact=True)).to_have_text("10")
    expect(rows.last.locator("td").last).to_have_text("10")
    session = page.request.get(BASE + "/api/session").json()
    headers = {"X-CSRF-Token": session["csrfToken"]}
    for path in ("reef-check-data/codes", "substrate-types"):
        response = page.request.get(BASE + "/api/admin/" + path, headers=headers)
        assert response.ok, (path, response.status)
        numbers = {row["code"]: row["numeric_code"] for row in response.json()}
        assert numbers["OT"] == 10 and numbers["NA"] == -1, path
    expect(down.locator("tbody td").nth(1)).to_have_text("1")

    bleaching = page.get_by_role("region", name="底質白化", exact=True)
    expect(bleaching.get_by_role("table")).to_have_count(2)
    expect(bleaching).to_contain_text("依各段填報")
    summary = bleaching.get_by_role("region", name="穿越線白化統計")
    expect(summary.get_by_role("row", name="硬珊瑚", exact=False).locator("td")).to_have_text(["1", "33.33%（1／3）", "0.5", "0.71", "0.5", "2/4"])
    expect(summary.get_by_role("row", name="軟珊瑚", exact=False).locator("td")).to_have_text(["0", "無法計算", "0", "0", "0", "3/4"])
    expect(bleaching.get_by_role("table").first.locator("tbody tr").nth(1).locator("td")).to_have_text(["1", "50%（1／2）", "0", "無表層 SC 點"])
    expect(bleaching.get_by_role("table").first.locator("tbody tr").last.locator("td").first).to_have_text("NA（未記錄）")

    page.set_viewport_size({"width": 390, "height": 844})
    region = page.get_by_role("region", name="表層底質手板")
    assert region.evaluate("el => el.clientWidth <= innerWidth && el.scrollWidth > el.clientWidth")
    region.evaluate("el => el.scrollLeft = el.scrollWidth")
    assert region.evaluate("el => el.scrollLeft > 0")
    page.set_viewport_size({"width": 1280, "height": 800})

    page.get_by_role("button", name="編輯這條穿越線").click()
    expect(surface.get_by_role("combobox")).to_have_count(7)
    page.get_by_label("表層第 4 段 94.5 m 底質", exact=True).select_option("RB")
    page.get_by_role("button", name="儲存修改").click()
    expect(rows.last.locator("td").last).to_have_text("7")
    page.get_by_role("button", name="返回場次列表", exact=False).click()
    page.get_by_role("button", name="查看 browser-member", exact=True).click()
    page.get_by_role("button", name="底質 Line", exact=True).click()
    expect(rows.last.locator("td").last).to_have_text("7")
    expect(down.locator("tbody td").nth(1)).to_have_text("1")
    page.get_by_role("button", name="返回場次列表", exact=False).click()
    page.get_by_role("button", name="查看 browser-other", exact=True).click()
    expect(bleaching.get_by_role("table")).to_have_count(2)
    expect(bleaching).to_contain_text("依逐格白化標記")
    summary = bleaching.get_by_role("region", name="穿越線白化統計")
    expect(summary.get_by_role("row", name="硬珊瑚", exact=False).locator("td")).to_have_text(['1', '50%（1／2）', '1', '—', '—', '1/4'])
    expect(summary.get_by_role("row", name="軟珊瑚", exact=False).locator("td")).to_have_text(['0', '0%（0／1）', '0', '0', '0', '2/4'])
    expect(bleaching.get_by_role("table").first.locator("tbody tr").first.locator("td")).to_have_text(["1", "50%（1／2）", "0", "0%（0／1）"])
    expect(bleaching.get_by_role("table").first.locator("tbody tr").nth(1)).to_contain_text("白化標記未齊")
    expect(bleaching.get_by_role("table").first.locator("tbody tr").nth(2)).to_contain_text("無有效底質點")
    expect(bleaching.get_by_role("table").first.locator("tbody tr").last).to_contain_text("本段未記錄")
    page.get_by_role("button", name="返回場次列表", exact=False).click()
    print("Chrome passed: historical totals and point marks, subtype denominator, valid zero and missing reasons.")
    print("Chrome passed: four-segment handboard, layer isolation, mobile scrolling and persisted point edit.")


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

        check_substrate_board(page)
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
        save.click()
        expect(page.locator(".pc-inline-notice")).to_contain_text("已儲存變更")
        verify(second, 200)
        page.get_by_role("button", name="重發代碼").click()
        expect(output).to_be_in_viewport()
        verify(output.inner_text(), 200)
        verify(second, 400)
        print("Chrome passed: diver search, method intersection, clear, required site, create, copy, volunteer verification, re-enable, reopen and reissue.")
        browser.close()


if __name__ == "__main__":
    seed()
    with subprocess.Popen([os.path.join(os.environ["RUNNER_TEMP"], "reef-browser", "server")], env={**os.environ, "HTTP_ADDR": "127.0.0.1:8080", "DATABASE_URL": DB, "SESSION_SECRET": secrets.token_hex(32), "ADMIN_ALLOWED_ORIGINS": BASE, "SECURE_COOKIES": "false", "ENABLE_CWA_CRON": "false"}) as server:
        try:
            for _ in range(120):
                try:
                    urllib.request.urlopen(BASE + "/api/session", timeout=1).close()
                    break
                except urllib.error.URLError:
                    time.sleep(0.5)
            else:
                raise RuntimeError("CI server failed to start")
            test_ui()
        finally:
            server.terminate()
