"""Remove form-filling directions from the embedded report templates.

Edit only the templates' static string table. Never filter text from an observation.
"""

from pathlib import Path
import re
from xml.etree import ElementTree as ET
from xml.sax.saxutils import escape
from zipfile import ZipFile
from tempfile import TemporaryDirectory

NS = {"s": "http://schemas.openxmlformats.org/spreadsheetml/2006/main"}
REPLACEMENTS = {
    "field-line": {
        "DATA": {
            "A6": "Surface substrate observations",
            "P36": "",
            "P37": "",
            "A49": "",
            "A63": "Number of recorded points per segment",
            "A65": "",
            "L65": "",
        }
    },
    "field-belt": {
        "DATA": {"A5": "Belt transect observations", "A67": "", "A69": "", "D69": ""}
    },
    "field-site": {
        "DATA": {"A1": "Site survey report"},
        "FIELD SHEET SITE": {"B1": "Site survey"},
    },
    "line": {
        "DATA": {
            "Q1": "手板代碼",
            "A11": "表層底質逐點紀錄",
            "A45": "",
            "A69": "各段有效測點數",
        },
        "EXPORT": {"A16": ""},
    },
    "belt": {
        "DATA": {
            "A5": "魚類、無脊椎與環境衝擊觀測紀錄",
            "H53": "",
            "A73": "",
            "A75": "",
            "D75": "",
        }
    },
}


def clean_report_labels(path):
    path = Path(path)
    if path.stem not in REPLACEMENTS:
        return
    with TemporaryDirectory(dir=path.parent) as tmp:
        dest = Path(tmp) / path.name
        with ZipFile(path) as src, ZipFile(dest, "w") as dst:
            wb = ET.fromstring(src.read("xl/workbook.xml"))
            rels = {
                r.get("Id"): r.get("Target")
                for r in ET.fromstring(src.read("xl/_rels/workbook.xml.rels"))
            }
            replacements = {}
            for sheet in wb.findall("s:sheets/s:sheet", NS):
                name = sheet.get("name")
                rid = sheet.get(
                    "{http://schemas.openxmlformats.org/officeDocument/2006/relationships}id"
                )
                xml = ET.fromstring(src.read("xl/" + rels[rid]))
                labels = REPLACEMENTS[path.stem].get(name, {}).copy()
                if name in ("GRAPHS", "GRAPHS_SI"):
                    labels.update(dict(E1="", E2="", A3=""))
                for cell in xml.findall(".//s:c[@t='s']", NS):
                    ref = cell.get("r")
                    if name in ("使用指引", "INSTRUCTIONS") and int(
                        re.search(r"\d+", ref).group()
                    ) < (40 if name == "使用指引" else 38):
                        labels[ref] = (
                            "報表說明"
                            if ref == "A1"
                            else "依所選場次的已儲存觀測資料產生。"
                            if ref == "A2"
                            else ""
                        )
                    if ref in labels:
                        replacements[int(cell.findtext("s:v", namespaces=NS))] = labels[
                            ref
                        ]
            for entry in src.infolist():
                data = src.read(entry)
                if entry.filename == "xl/sharedStrings.xml":
                    indices = iter(range(len(ET.fromstring(data))))

                    def replace(match):
                        idx = next(indices)
                        if idx not in replacements:
                            return match.group()
                        return "<si><t>" + escape(replacements[idx]) + "</t></si>"

                    data = re.sub(
                        r"<si(?:\s[^>]*)?>.*?</si>", replace, data.decode(), flags=re.S
                    ).encode()
                dst.writestr(entry, data)
        dest.replace(path)
