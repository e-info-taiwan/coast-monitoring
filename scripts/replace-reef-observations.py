#!/usr/bin/env python3
"""Validate the pinned handoff, rehearse on a restored DB, then optionally replace dev.

Run in the user's normal Terminal (gcloud user credentials are required).
No credentials are written to logs or command arguments. Backups contain private data.
"""
import argparse
import csv
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import socket
import subprocess
import sys
import time
from urllib.parse import parse_qs, quote, unquote, urlsplit
import zipfile

ROOT = Path(__file__).resolve().parents[1]
ARCHIVE = "coral_data_RD_handoff_2026-11-04_v1.zip"
PACKAGE = "2026-11-04_coral_data_handoff_v1"
PINNED_SHA256 = "deedd8ea854b7cb246ed8beaf3f5bc2c89339664a79ffd52e74651789feb285a"
PROJECT, REGION, SERVICE = "tnf-rc", "asia-east1", "coast-monitoring-dev"
TABLES = ["survey", "event", "transect", "transect_participant", "substrate_point",
          "substrate_bleaching", "belt_observation", "impact_observation", "incidental_sighting"]


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def run(args, *, env=None, capture=False, stdout=None):
    result = subprocess.run([str(a) for a in args], cwd=ROOT, env=env,
                            stdout=subprocess.PIPE if capture else stdout,
                            stderr=subprocess.PIPE, text=True)
    if result.returncode:
        detail = result.stderr
        for key in ["DATABASE_URL", "PGPASSWORD"]:
            value = (env or {}).get(key)
            if value:
                detail = detail.replace(value, "[redacted]")
        raise RuntimeError(f"{Path(str(args[0])).name} failed (exit {result.returncode}); no later steps were run\n{detail[-2000:]}")
    return result.stdout if capture else None


def read_csv(path):
    with path.open(encoding="utf-8-sig", newline="") as f:
        return list(csv.DictReader(f))


def unique(rows, columns, label):
    keys = [tuple(r[c] for c in columns) for r in rows]
    require(len(keys) == len(set(keys)), f"duplicate natural key: {label}")


def check_package(destination):
    archive = ROOT / ARCHIVE
    expected = (ROOT / (ARCHIVE + ".sha256")).read_text().split()[0]
    require(re.fullmatch(r"[0-9a-f]{64}", expected), "invalid SHA256 file")
    actual = hashlib.sha256(archive.read_bytes()).hexdigest()
    require(actual == expected, "ZIP checksum mismatch")
    # Pin the reviewed delivery independently of the adjacent checksum file.
    require(actual == PINNED_SHA256, "this runner only accepts the reviewed 2026-11-04 v1 delivery")
    with zipfile.ZipFile(archive, metadata_encoding="utf-8") as z:
        for item in z.infolist():
            target = (destination / item.filename).resolve()
            require(target.is_relative_to(destination.resolve()), "unsafe archive path")
            require((item.external_attr >> 16) & 0o170000 != 0o120000, "symlinks not accepted")
        z.extractall(destination)
    package = destination / PACKAGE
    for checksums in package.rglob("SHA256SUMS.txt"):
        for line in checksums.read_text().splitlines():
            if not line.strip():
                continue
            digest, name = line.split(maxsplit=1)
            file = (checksums.parent / name.lstrip("*")).resolve()
            require(file.is_relative_to(package.resolve()), "unsafe checksum path")
            require(hashlib.sha256(file.read_bytes()).hexdigest() == digest, f"checksum mismatch: {file.name}")
    counts = json.loads((package / "delivery_manifest.json").read_text())["modules"]["reefcheck_observations"]["row_counts"]
    csv_dir = package / "01_reefcheck_observations" / "csv"
    data = {name: read_csv(csv_dir / name) for name in counts}
    for name, count in counts.items():
        require(len(data[name]) == count, f"row count mismatch: {name}")
    keys = {"site_official_lookup": ["dive_site__lookup"], "diver": ["diver_key"],
            "survey": ["survey_key"], "event": ["event_id"], "transect": ["transect_key"],
            "transect_participant": ["transect_key", "diver_key", "role"],
            "substrate_point": ["transect_key", "position_m", "substrate_layer"],
            "substrate_bleaching": ["transect_key", "segment"],
            "belt_observation": ["transect_key", "taxon_group", "taxon_name_en__lookup", "taxon_size_class__lookup", "segment"],
            "impact_observation": ["transect_key", "impact_group", "impact_name_en__lookup", "segment"],
            "incidental_sighting": ["transect_key", "source_file", "source_sheet", "source_cell"]}
    for table, columns in keys.items():
        unique(data[table + ".csv"], columns, table)
    unique(data["survey.csv"], ["site_name_en__lookup", "start_date"], "survey DB key")
    unique(data["event.csv"], ["event_key"], "event_key")
    unique(data["transect.csv"], ["event_id", "method"], "transect DB key")
    sites = {r["dive_site__lookup"] for r in data["site_official_lookup.csv"]}
    surveys = {r["survey_key"] for r in data["survey.csv"]}
    events = {r["event_id"] for r in data["event.csv"]}
    transects = {r["transect_key"]: r for r in data["transect.csv"]}
    divers = {r["diver_key"] for r in data["diver.csv"]}
    require(all(r["site_name_en__lookup"] in sites for r in data["survey.csv"]), "survey site FK mismatch")
    require(all(r["survey_key"] in surveys for r in data["event.csv"]), "event survey FK mismatch")
    require(all(r["event_id"] in events for r in transects.values()), "transect event FK mismatch")
    for table in TABLES[3:]:
        for r in data[table + ".csv"]:
            require(r["transect_key"] in transects, f"transect FK mismatch: {table}")
            method = transects[r["transect_key"]]["method"]
            if table.startswith("substrate_"):
                require(method == "line", "substrate method mismatch")
            if table == "impact_observation":
                require(method == "belt_invert", "impact method mismatch")
            if table == "belt_observation":
                require(method == ("belt_fish" if r["taxon_group"] == "fish" else "belt_invert"), "belt method mismatch")
            if table == "transect_participant":
                require(r["diver_key"] in divers, "participant diver FK mismatch")
    report = {"archive_sha256": actual, "counts": counts,
              "belt_null_count": sum(r["count"] == "" for r in data["belt_observation.csv"]),
              "impact_units": {u: sum(r["source_unit"] == u for r in data["impact_observation.csv"])
                               for u in ["count", "level", "percent"]}}
    (destination / "package-validation.json").write_text(json.dumps(report, ensure_ascii=False, indent=2))
    print(json.dumps(report, ensure_ascii=False, indent=2), flush=True)
    return csv_dir


def free_port():
    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


def pg_env(user, password, database, port):
    env = {k: v for k, v in os.environ.items() if not k.startswith("PG") and k != "DATABASE_URL"}
    env.update(PGHOST="127.0.0.1", PGPORT=str(port), PGUSER=user, PGPASSWORD=password,
               PGDATABASE=database, PGSSLMODE="disable", PGCONNECT_TIMEOUT="10")
    env["DATABASE_URL"] = f"postgresql://{quote(user, safe='')}:{quote(password, safe='')}@127.0.0.1:{port}/{quote(database, safe='')}?sslmode=disable"
    return env


def cloud_target(app_commit=None):
    service = json.loads(run(["gcloud", "run", "services", "describe", SERVICE, "--project", PROJECT,
                              "--region", REGION, "--format=json"], capture=True))
    template = service["spec"]["template"]
    if app_commit:
        require(re.fullmatch(r"[0-9a-f]{7,40}", app_commit), "invalid application commit")
        image = template["spec"]["containers"][0]["image"]
        tag = image.rsplit(":", 1)[-1]
        require(len(tag) >= 7 and (tag.startswith(app_commit) or app_commit.startswith(tag)),
                "compatible application image is not deployed yet; wait for Cloud Build")
        status = service["status"]
        ready = status.get("latestReadyRevisionName")
        require(ready and ready == status.get("latestCreatedRevisionName"), "latest application revision is not ready")
        require(sum(t.get("percent", 0) for t in status.get("traffic", []) if t.get("revisionName") == ready) == 100,
                "compatible revision is not serving 100% of traffic")
    instances = template["metadata"]["annotations"]["run.googleapis.com/cloudsql-instances"].split(",")
    require(len(instances) == 1 and instances[0].startswith(PROJECT + ":" + REGION + ":"), "ambiguous Cloud SQL instance")
    instance = instances[0]
    containers = template["spec"]["containers"]
    db_entries = [e for c in containers for e in c.get("env", []) if e["name"] == "DATABASE_URL"]
    require(len(db_entries) == 1, "ambiguous DATABASE_URL")
    entry = db_entries[0]
    require("valueFrom" in entry, "expected DATABASE_URL Secret Manager reference")
    ref = entry["valueFrom"]["secretKeyRef"]
    secret_name = ref["name"]
    aliases = template["metadata"]["annotations"].get("run.googleapis.com/secrets",
               service.get("metadata", {}).get("annotations", {}).get("run.googleapis.com/secrets", ""))
    aliases = dict(pair.split(":", 1) for pair in aliases.split(",") if ":" in pair)
    secret_name = aliases.get(secret_name, secret_name)
    if secret_name.startswith("projects/"):
        parts = secret_name.split("/")
        secret_project, secret_name = parts[1], parts[3]
    else:
        secret_project = PROJECT
    secret = run(["gcloud", "secrets", "versions", "access", ref["key"], "--secret", secret_name,
                  "--project", secret_project], capture=True).strip()
    parsed = urlsplit(secret)
    require(parsed.scheme in ("postgres", "postgresql"), "DATABASE_URL must be a PostgreSQL URI")
    query = parse_qs(parsed.query)
    host = query.get("host", [parsed.hostname or ""])[0]
    description = json.loads(run(["gcloud", "sql", "instances", "describe", instance.split(":")[-1],
                                  "--project", PROJECT, "--format=json"], capture=True))
    allowed = {"/cloudsql/" + instance} | {x["ipAddress"] for x in description.get("ipAddresses", [])}
    require(host in allowed, "secret host does not match the service's Cloud SQL instance")
    user = query.get("user", [unquote(parsed.username or "")])[0]
    password = query.get("password", [unquote(parsed.password or "")])[0]
    database = unquote(parsed.path.lstrip("/"))
    require(user and password and database, "incomplete database credentials")
    print(f"Target: {PROJECT} / {SERVICE} / {instance} / database {database}", flush=True)
    return instance, user, password, database


def sql(env, statement):
    return run(["psql", "-X", "-v", "ON_ERROR_STOP=1", "-At", "-c", statement], env=env, capture=True).strip()


def fingerprints(env, tables):
    return {t: sql(env, "SELECT md5(COALESCE(string_agg(md5(row_to_json(r)::text),'' ORDER BY id),'')) FROM " + t + " r")
            for t in tables}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check-package", action="store_true", help="offline validation only")
    parser.add_argument("--apply", action="store_true", help="replace online observations after successful local rehearsal")
    parser.add_argument("--app-commit", help="required with --apply: deployed commit containing source-unit query support")
    args = parser.parse_args()
    require(not args.apply or args.app_commit, "--apply requires --app-commit for the compatible deployed backend")
    require(sys.version_info >= (3, 11), "Python 3.11 or newer is required")
    os.umask(0o077)
    os.chdir(ROOT)
    work = ROOT / ".local" / "backups" / ("reef-replacement-" + time.strftime("%Y%m%d-%H%M%S"))
    work.mkdir(parents=True, exist_ok=False)
    print(f"Work and backup directory: {work}", flush=True)
    csv_dir = check_package(work)
    if args.check_package:
        return
    for name in ["gcloud", "cloud-sql-proxy", "pg_dump", "pg_restore", "psql", "initdb", "pg_ctl", "createdb", "go"]:
        require(shutil.which(name), f"required executable missing: {name}")
    importer, migrator = work / "import-reefcheck", work / "migrate"
    run(["go", "build", "-o", importer, "./cmd/import-reefcheck"])
    run(["go", "build", "-o", migrator, "./cmd/migrate"])
    instance, user, password, database = cloud_target(args.app_commit)
    port = free_port()
    live = pg_env(user, password, database, port)
    proxy_log = (work / "proxy.log").open("w")
    proxy = subprocess.Popen(["cloud-sql-proxy", "--gcloud-auth", "--address=127.0.0.1", f"--port={port}", instance],
                             stdout=proxy_log, stderr=proxy_log)
    stage_started = False
    pgdata = work / "staging-pg"
    try:
        for _ in range(60):
            require(proxy.poll() is None, "Cloud SQL proxy failed; see the private proxy.log")
            try:
                with socket.create_connection(("127.0.0.1", port), timeout=1):
                    break
            except OSError:
                time.sleep(1)
        else:
            raise RuntimeError("Cloud SQL proxy startup timed out")
        require(sql(live, "SELECT current_database()") == database, "database identity mismatch")
        before = sql(live, "SELECT count(*) FROM event")
        print(f"Current online events: {before}. Creating full backup for staging...", flush=True)
        backup = work / "before-migrations.dump"
        run(["pg_dump", "-Fc", "--no-owner", "--no-acl", "-f", backup], env=live)
        run(["pg_restore", "--list", backup], capture=True)
        # Restore the actual live schema/data to an isolated local server before touching live rows.
        stage_port = free_port()
        stage = pg_env("coast_stage", "", "coast_stage", stage_port)
        run(["initdb", "-D", pgdata, "-U", "coast_stage", "-A", "trust", "--no-locale", "-E", "UTF8"], capture=True)
        # A short socket path also avoids macOS's Unix socket path length limit.
        run(["pg_ctl", "-D", pgdata, "-l", work / "staging.log", "-o",
             f"-h 127.0.0.1 -p {stage_port} -k ''", "-w", "start"], capture=True)
        stage_started = True
        run(["createdb", "coast_stage"], env=stage)
        run(["pg_restore", "--exit-on-error", "--no-owner", "--no-acl", "-d", "coast_stage", backup], env=stage)
        run([migrator], env=stage)
        base = [importer, "--csv-dir", csv_dir, "--replace-observations"]
        stage_before = fingerprints(stage, TABLES)
        protected = fingerprints(stage, ["users", "reef_check_activity", "public_content"])
        print("Rehearsing replacement with rollback on local restored database...", flush=True)
        run(base + ["--dry-run"], env=stage)
        require(fingerprints(stage, TABLES) == stage_before, "dry-run did not preserve observation rows")
        # Fail near the end of the import, after DELETE and most INSERTs have run.
        broken = work / "invalid-csv-test"
        shutil.copytree(csv_dir, broken)
        impact_file = broken / "impact_observation.csv"
        invalid_rows = read_csv(impact_file)
        invalid_rows[-1]["raw_value"] = "-1"
        with impact_file.open("w", encoding="utf-8-sig", newline="") as f:
            writer = csv.DictWriter(f, fieldnames=list(invalid_rows[0]))
            writer.writeheader()
            writer.writerows(invalid_rows)
        try:
            run([importer, "--csv-dir", broken, "--replace-observations", "--dry-run"], env=stage)
        except RuntimeError:
            pass
        else:
            raise RuntimeError("invalid impact input was accepted")
        require(fingerprints(stage, TABLES) == stage_before, "failed import did not restore original observations")
        print("Rehearsing committed replacement and locked backup locally...", flush=True)
        run(base + ["--backup-file", work / "staging-before-replacement.dump"], env=stage)
        require(fingerprints(stage, ["users", "reef_check_activity", "public_content"]) == protected,
                "replacement changed accounts, activities or content settings")
        require(sql(stage, "SELECT count(*) FROM event") == "757", "staging event count mismatch")
        require(sql(stage, "SELECT count(*) FROM impact_observation_with_level WHERE source_unit='level' AND derived_level IS DISTINCT FROM raw_value") == "0", "historical impact level conversion mismatch")
        if not args.apply:
            print("Rehearsal passed. Online observations unchanged. Run with --apply to replace.", flush=True)
            return
        print("Rehearsal passed. Applying schema and atomically replacing online observations...", flush=True)
        run([migrator], env=live)
        run(base + ["--backup-file", work / "online-before-replacement.dump"], env=live)
        summary = {t: int(sql(live, "SELECT count(*) FROM " + t)) for t in TABLES}
        (work / "online-result.json").write_text(json.dumps(summary, indent=2))
        print("Online replacement committed and verified:\n" + json.dumps(summary, indent=2), flush=True)
        print(f"Backups and verification: {work}", flush=True)
    finally:
        if stage_started:
            subprocess.run(["pg_ctl", "-D", str(pgdata), "-m", "fast", "-w", "stop"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        proxy.terminate()
        try:
            proxy.wait(timeout=10)
        except subprocess.TimeoutExpired:
            proxy.kill()
            proxy.wait()
        proxy_log.close()


if __name__ == "__main__":
    try:
        main()
    except (RuntimeError, OSError, ValueError, KeyError) as exc:
        print(f"STOP: {exc}", file=sys.stderr)
        sys.exit(1)
