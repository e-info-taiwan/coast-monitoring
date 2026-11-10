"""Build the three blank field workbooks from the specified 2025 Yeliu XLS files."""

import argparse
import hashlib
from datetime import datetime
from pathlib import Path
import struct
import xlrd
import xlsxwriter
from xlsxwriter.utility import xl_rowcol_to_cell


# Only retain labels, legends, distances and empty field sheets.
def retain(kind, sheet, r, c, address):
    if sheet == "GRAPHS":
        return (
            c == 0
            or r + 1
            in ({4, 19, 26} if kind == "line" else {4, 15, 21, 32, 40, 47, 51, 55})
            or c == 4
        )
    if sheet != "DATA":
        return True
    if kind == "site":
        return (
            c == 0
            or address
            in "D6 G6 C7 D7 G7 C8 E8 G8 I8 C9 E9 G9 I9 B11 D11 F11 H11 J11 D13 E13 I13 D15 E15 D18 E18 G24 F42 E53".split()
        )
    if kind == "belt":
        return c == 0 or r + 1 in [9, 28, 49, 69] or (c == 4 and r < 3)
    if 15 <= r + 1 <= 34:
        return c % 2 == 0
    if 52 <= r + 1 <= 62:
        return c % 2 == 0 and c <= 10 and (r + 1 != 62 or c < 8)
    if r < 4:
        return c in [0, 8]
    if 36 <= r + 1 <= 40:
        return c == 0 or c == 15
    return True


SOURCE_HASHES = {
    "line": "ba3228a327b6e4ab1415f5be899f77341e7198eaf19e93ef76f051957e83f1ab",
    "belt": "d3d4a567aa2573f7653b61353d268937bcec1e4f4ebf72820e9083cd58b62655",
    "site": "37708006bef7d512e98bda8d8c3ca942ab0cf0bbafe15dd98c5dc5c5af6f392b",
}


def make(kind, source_path, output_dir):
    if hashlib.sha256(source_path.read_bytes()).hexdigest() != SOURCE_HASHES[kind]:
        raise ValueError(
            f"Unexpected {kind} workbook. Recheck input ranges before embedding a new survey form."
        )
    source = xlrd.open_workbook(str(source_path), formatting_info=True, on_demand=True)
    target = xlsxwriter.Workbook(
        output_dir / ("field-" + kind + ".xlsx"),
        {"strings_to_urls": False, "strings_to_formulas": False},
    )
    target.set_properties({"created": datetime(2025, 7, 17)})
    styles = {}

    def color(i):
        rgb = source.colour_map.get(i)
        return "#%02X%02X%02X" % rgb if rgb else None

    def fmt(i):
        if i in styles:
            return styles[i]
        x = source.xf_list[i]
        f = source.font_list[x.font_index]
        a = x.alignment
        props = dict(
            font_name=f.name,
            font_size=f.height / 20,
            bold=bool(f.bold),
            italic=bool(f.italic),
            text_wrap=bool(a.text_wrapped),
            num_format=source.format_map[x.format_key].format_str,
        )
        if color(f.colour_index):
            props["font_color"] = color(f.colour_index)
        if a.hor_align in (1, 2, 3, 6):
            props["align"] = {1: "left", 2: "center", 3: "right", 6: "center_across"}[
                a.hor_align
            ]
        props["valign"] = {0: "top", 1: "vcenter", 2: "bottom"}.get(
            a.vert_align, "bottom"
        )
        for side in ("left", "right", "top", "bottom"):
            props[side] = getattr(x.border, side + "_line_style")
            col = color(getattr(x.border, side + "_colour_index"))
            if col:
                props[side + "_color"] = col
        if x.background.fill_pattern == 1 and color(x.background.pattern_colour_index):
            props["bg_color"] = color(x.background.pattern_colour_index)
        styles[i] = target.add_format(props)
        return styles[i]

    for idx, s in enumerate(source.sheets()):
        sheet = target.add_worksheet(s.name)
        rows, cols = {
            "line": [(67, 19), (45, 15), (54, 2), (45, 24)],
            "belt": [(71, 12), (116, 16), (54, 2), (29, 11), (48, 11)],
            "site": [(58, 10), (54, 13)],
        }[kind][idx]
        for c in range(cols):
            sheet.set_column_pixels(
                c, c, round(s.computed_column_width(c) / 256 * 7 + 5)
            )
        for r in range(rows):
            if r in s.rowinfo_map:
                sheet.set_row(r, s.rowinfo_map[r].height / 20)
        for r1, r2, c1, c2 in s.merged_cells:
            if r2 <= rows and c2 <= cols:
                sheet.merge_range(
                    r1, c1, r2 - 1, c2 - 1, "", fmt(s.cell_xf_index(r1, c1))
                )
        for r in range(min(rows, s.nrows)):
            for c in range(min(cols, s.ncols)):
                cell = s.cell(r, c)
                addr = xl_rowcol_to_cell(r, c)
                if cell.ctype == xlrd.XL_CELL_EMPTY:
                    continue
                v = cell.value
                if cell.ctype in (xlrd.XL_CELL_BLANK, xlrd.XL_CELL_ERROR) or not retain(
                    kind, s.name, r, c, addr
                ):
                    v = None
                style = fmt(cell.xf_index)
                if (
                    kind == "belt"
                    and s.name == "DATA"
                    and r + 1 in (55, 56, 58, 59)
                    and c in (6, 7)
                ):
                    style = fmt(s.cell_xf_index(r, 1))
                sheet.write(r, c, v, style)
        if s.name == "GRAPHS":
            # Rectangles are the original two-cell chart anchors, in worksheet order.
            if kind == "line":
                charts = [
                    (5, 14, "Substrate cover", "column", (8, 5, 21, 11)),
                    (20, 24, "Live reef cover", "column", (23, 5, 34, 8)),
                    (27, 31, "Non-living reef cover", "column", (23, 8, 35, 11)),
                    (20, 24, "Live reef cover", "pie", (37, 5, 45, 8)),
                    (27, 31, "Non-living reef cover", "pie", (37, 8, 45, 11)),
                ]
            else:
                charts = [
                    (5, 13, "Fish", "column", (9, 5, 24, 11)),
                    (16, 19, "Grouper", "column", (32, 5, 47, 11)),
                    (22, 30, "Invertebrates", "column", (62, 5, 67, 11)),
                    (33, 38, "Giant clam", "column", (67, 5, 72, 11)),
                    (41, 45, "Impacts", "column", (78, 6, 84, 10)),
                    (48, 49, "Bleaching – population", "pie", (84, 5, 98, 8)),
                    (52, 53, "Bleaching – colony", "pie", (84, 8, 98, 11)),
                    (56, 57, "Coral disease", "column", (100, 6, 115, 10)),
                ]
            for a, b, label, chart_type, (top, left, bottom, right) in charts:
                chart = target.add_chart({"type": chart_type})
                series = {
                    "name": label,
                    "categories": ["GRAPHS", a - 1, 0, b - 1, 0],
                    "values": ["GRAPHS", a - 1, 1, b - 1, 1],
                }
                if a < 48 and chart_type != "pie":
                    se = f"=GRAPHS!$C${a}:$C${b}"
                    series["y_error_bars"] = {
                        "type": "custom",
                        "plus_values": se,
                        "minus_values": se,
                    }
                chart.add_series(series)
                chart.set_title({"name": label})
                if chart_type == "pie":
                    chart.set_legend({"position": "bottom"})
                else:
                    chart.set_legend({"none": True})
                width = sum(
                    s.computed_column_width(c) / 256 * 7 + 5
                    for c in range(left - 1, right - 1)
                )
                height = sum(
                    (
                        s.rowinfo_map[r].height
                        if r in s.rowinfo_map
                        else s.default_row_height
                    )
                    / 15
                    for r in range(top - 1, bottom - 1)
                )
                chart.set_size({"width": round(width), "height": round(height)})
                sheet.insert_chart(top - 1, left - 1, chart)
        # Display the original survey choices as printable empty boxes.
        if kind == "site":
            p = source._sh_abs_posn[idx]
            prev = None
            anchor = None
            while True:
                tag, n = struct.unpack_from("<HH", source.mem, p)
                data = source.mem[p + 4 : p + 4 + n]
                p += n + 4
                off = data.rfind(bytes.fromhex("000010f012000000"))
                if off >= 0:
                    _, c, dx, r, dy, *_ = struct.unpack_from("<9H", data, off + 8)
                    anchor = (r, c, dx, dy)
                if prev == 0x1B6 and tag == 60 and anchor:
                    v = data[1:].decode("utf-16-le" if data[0] else "latin1").strip()
                    if v:
                        r, c, dx, dy = anchor
                        # Lower-edge anchors belong to the next row.
                        if dy >= 120:
                            r += 1
                        if idx == 0 and v in ("sunny", "cloudy", "raining"):
                            continue
                        x = round(
                            dx / 1024 * (s.computed_column_width(c) / 256 * 7 + 5)
                        )
                        sheet.insert_textbox(
                            r,
                            c,
                            "□ " + v,
                            {
                                "x_offset": x,
                                "width": max(46, len(v) * 6 + 12),
                                "height": 15,
                                "font": {"size": 8},
                                "fill": {"none": True},
                                "line": {"none": True},
                                "text_options": {"vertical_alignment": "middle"},
                                "align": {"vertical": "middle"},
                                "object_position": 1,
                            },
                        )
                prev = tag
                if tag == 10:
                    break
        sheet.set_paper(9)
        sheet.fit_to_pages(1, 0 if s.name == "GRAPHS" else 1)
        if s.name == "INSTRUCTIONS":
            sheet.set_column(0, 0, 120)
        sheet.print_area(0, 4 if s.name == "GRAPHS" else 0, rows - 1, cols - 1)
        if kind == "line" or s.name == "GRAPHS":
            sheet.set_landscape()
    target.close()


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    for kind in SOURCE_HASHES:
        parser.add_argument(kind, type=Path)
    args = parser.parse_args()
    output = Path(__file__).resolve().parents[1] / "internal/report/templates"
    for kind in SOURCE_HASHES:
        make(kind, getattr(args, kind), output)
