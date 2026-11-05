import { test } from "node:test"
import assert from "node:assert/strict"
import fs from "node:fs/promises"
import { PGlite } from "@electric-sql/pglite"
import { pgcrypto } from "@electric-sql/pglite/contrib/pgcrypto"
import { citext } from "@electric-sql/pglite/contrib/citext"
test("upgrade preserves legacy activity identity, report URLs, hidden charts and all group depths", async () => {
  const db = new PGlite({ extensions: { pgcrypto, citext } })
  await db.waitReady
  const dir = new URL("../../migrations/", import.meta.url)
  try {
    for (const file of (await fs.readdir(dir)).filter((f) => f.endsWith(".sql")).sort()) {
      if (file.startsWith("000018"))
        await db.exec(`
   INSERT INTO site(id,name_zh) VALUES(10001,'Upgrade A'),(10002,'Upgrade B');
   INSERT INTO public_content(kind,data) VALUES('activity','{"title":"same name"}');
   INSERT INTO reef_check_activity(name,code_hash,start_date,end_date,is_active) VALUES('same name','fixture-hash','2026-01-01','2026-01-02',false);
   INSERT INTO reef_check_activity_site(activity_id,site_id) SELECT id,10001 FROM reef_check_activity;
   INSERT INTO public_content(kind,site_id,publication_status,data) VALUES('profile',10001,'published','{"title":"A report","report_url":"https://example.org/original.pdf"}');
   INSERT INTO public_content(kind,site_id,publication_status,data) VALUES('chart',10001,'published','{"title":"hidden","chart":"taxa","is_visible":false,"depths_m":[5,10],"category_keys":["2","3"]}');
   INSERT INTO public_content(kind,publication_status,data) VALUES('chart','published','{"title":"shared","chart":"live_coral","is_visible":true,"site_ids":[10001,10002],"depths_m":[5,10]}');
  `)
      await db.exec("BEGIN;" + (await fs.readFile(new URL(file, dir), "utf8")) + ";COMMIT;")
    }
    const rows = async (q) => (await db.query(q)).rows
    assert.equal((await rows(`SELECT * FROM public_content WHERE kind='activity'`)).length, 2)
    const [entry] = await rows(`SELECT * FROM reef_check_activity`)
    assert.equal(entry.is_active, false)
    assert(entry.activity_content_id)
    const [report] = await rows(
      `SELECT c.data FROM public_content c JOIN article_site a ON a.article_id=c.id WHERE a.site_id=10001`,
    )
    assert.equal(report.data.external_url, "https://example.org/original.pdf")
    assert.equal(report.data.date, null)
    const [chart] = await rows(`SELECT * FROM site_chart_setting WHERE site_id=10001`)
    assert.equal(chart.is_visible, false)
    const [rare]=await rows(`SELECT * FROM site_chart_setting WHERE site_id=10001 AND chart_key='rare_history'`);assert.equal(rare.is_visible,false)
    assert.deepEqual(chart.category_keys, ["2", "3"])
    assert.equal((await rows(`SELECT * FROM shared_chart_member`)).length, 4)
    const review = await rows(
      `SELECT source_data FROM reef_prd_migration_review WHERE kind='chart' AND source_data->>'site_id'='10001'`,
    )
    assert.deepEqual(review[0].source_data.data.depths_m, [5, 10])
    await assert.rejects(
      db.exec(
        `INSERT INTO reef_check_activity(name,code_hash,start_date,end_date) VALUES('invalid','invalid','2026-01-01','2026-01-02')`,
      ),
      /not-null/,
    )
    await assert.rejects(db.exec(`INSERT INTO shared_chart_member SELECT * FROM shared_chart_member LIMIT 1`), /unique/)
  } finally {
    await db.close()
  }
})
