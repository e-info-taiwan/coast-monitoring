# RC Website Requirements To Current Design Mapping

> Status update (2026-07-20): this document began as a pre-implementation gap analysis.
> The active `reef_check_*` workflow now covers complete four-segment field entry, 160
> substrate points, recorders, bleaching, taxon/impact counts, survey CRUD, and computed
> per-survey reports. Historical cross-year charts and event annotations remain a separate
> reporting milestone. Migration `000004_reef_check_v12_schema.sql` is a target schema and
> is not yet the persistence path used by the application API.

## Sources

- `/Users/hcchien/Downloads/RC網站規劃資料.pdf`
- `/Users/hcchien/Downloads/RC網站規劃細節.pdf`
- Current implementation in this repository:
  - `migrations/000001_init.sql`
  - `internal/http/dto.go`
  - `web/admin/app.js`
  - `docs/api.md`

## Current Design Summary

The current system is a generic coast monitoring MVP.

| Area | Current design |
| --- | --- |
| Identity | `users`: email, name, role, status, Google/password login. Roles are `admin` and `volunteer`. |
| Site catalog | `locations`: only `chinese_name` and `english_name`. |
| Species catalog | `species`: only `chinese_name` and `english_name`. |
| Observation record | `observations`: `observed_on`, `location_id`, `species_id`, `observer_id`, `count`, `notes`. |
| Admin UI | Manages users, locations, species, observations, audit logs. |
| Field entry UI | Lets a volunteer choose date/location, then enter one count per species. Empty or zero counts are skipped. |
| Reporting | No Reef Check calculations, historical chart data, SD, SE, or live coral cover reporting. |

Important current constraints:

- `observations` is one species count at one location/date by one observer.
- There is no survey event/session entity.
- There is no site depth, GPS coordinate, county, transect segment, sub-sample area, substrate point, impact type, or rare organism structure.
- Recorders for benthos/fish/invertebrate are not represented separately.

## High-Level Fit

| PDF requirement group | Current fit | Current mapping | Gap |
| --- | --- | --- | --- |
| User/login roles | Mostly covered | `users.role`: `admin` / `volunteer` | Does not model survey team roles such as benthos recorder, fish recorder, invert recorder. |
| Sample basic data | Partially covered | `locations.chinese_name`, `locations.english_name`; `observations.observed_on` | Missing county, site, site English, N, E, depth, survey event, year/month/day output fields, sample dropdown metadata. |
| Benthos/substrate data | Not covered | Could only be forced into `species` + `observations`, which is not appropriate | Missing 4 sub-sample areas, 40 point codes per area, code validation, bleaching HC/SC counts, RKC reason trigger, substrate categories. |
| Fish counts | Partially covered | `species` catalog + `observations.count` | Missing survey segment columns `0-20m`, `25-45m`, `50-70m`, `75-95m`, fish recorder, optional body-length mode, fixed Reef Check fish categories and size classes. |
| Invertebrate counts | Partially covered | `species` catalog + `observations.count` | Missing segment-level structure, invertebrate recorder, fixed category list and size classes. |
| Environmental impact counts | Not covered | Could be stored as species-like rows, but semantics would be wrong | Missing impact type catalog, per-segment counts, 0-3 grading rule, averages/SD/SE. |
| Rare organisms | Partially covered | Could add sharks/turtles/mantas to `species` and use observations | Missing rare organism module, segment-level counts, `other` and rare-organism comments. |
| Comments | Partially covered | `observations.notes` | Missing survey-level comments, module-level comments, coral damage other explanation, RKC reason. |
| Historical reporting | Not covered | No reporting schema/API | Missing annual merge workflow, historical time series, typhoon marker, calculations. |

## Detailed Field Mapping

### Survey Basic Information

| Required field/planning item | Current design | Status | Recommended target |
| --- | --- | --- | --- |
| `year`, `month`, `day` | `observations.observed_on` stores one date | Partial | Store date on a new `reef_check_surveys.survey_date`; derive year/month/day for export. |
| `county` | None | Missing | Add to site/survey metadata. Prefer site catalog if stable. |
| `location` | `locations.chinese_name` | Partial | Keep `locations`, but clarify whether it means broad location or Reef Check site group. |
| `site` | None | Missing | Add `sites` or extend locations with `site_name`. |
| `site(English)` | `locations.english_name` may be reused | Partial | If location and site are distinct, add `sites.english_name`. |
| `N`, `E` | None | Missing | Add latitude/longitude fields to site catalog or survey snapshot. |
| `depth(m)` | None | Missing | Add depth to survey/site; allow repeated survey at same site with different depth. |
| Sample name dropdown | Location dropdown exists in UI | Partial | Back dropdown by site/depth catalog, not just location name. |
| Severe typhoon impact marker for historical chart | None | Missing | Add event/annotation table for chart overlays, or survey-level `environment_event_id`. |

### Benthos/Substrate Module

| Required field/planning item | Current design | Status | Recommended target |
| --- | --- | --- | --- |
| Four sub-sample areas | None | Missing | Add `reef_check_segments` with index 1-4 and labels. |
| Segment labels `0-20m`, `25-45m`, `50-70m`, `75-95m` | None | Missing | Store segment start/end meters or canonical label. |
| 40 substrate entries per sub-sample area | None | Missing | Add `substrate_points`: `survey_id`, `segment_index`, `point_index`, `code`. |
| Code validation | None | Missing | Add `substrate_codes` catalog with valid input codes and normalized category. |
| Input in 5-cell chunks, user presses `>>` to continue | None | Missing | UI behavior only; needs front-end state for chunked entry. |
| HC bleaching count per sub-sample area | None | Missing | Add `substrate_bleaching_counts.hc_bleached_count`. |
| SC bleaching count per sub-sample area | None | Missing | Add `substrate_bleaching_counts.sc_bleached_count`. |
| RKC proportion `>= 10%` prompts possible reason | None | Missing | Compute from substrate points and require `rkc_reason` when threshold is met. |
| Other comments after substrate completion | `observations.notes` only | Partial | Add survey/module-level comments. |

Substrate code catalog from the PDF:

| Input code | Category | Meaning |
| --- | --- | --- |
| `1` | `HC` | Hard coral |
| `2` | `SC` | Soft coral |
| `3` | `RKC` | Recently killed coral |
| `4` | `NIA` | Algae |
| `5` | `SP` | Sponge |
| `6` | `RC` | Rock |
| `7` | `RB` | Rubble |
| `8` | `SD` | Sand |
| `9` | `SI` | Silt |
| `0` | `OT` | Other |
| `a` | `HC-a` | Hard coral subtype |
| `b` | `HC-b` | Hard coral subtype |
| `c` | `HC-c` | Hard coral subtype |
| `91` | `SI(HC)` | Silt over hard coral |
| `92` | `SI(SC)` | Silt over soft coral |
| `93` | `SI(RKC)` | Silt over recently killed coral |
| `94` | `SI(NIA)` | Silt over algae |
| `95` | `SI(SP)` | Silt over sponge |
| `96` | `SI(RC)` | Silt over rock |
| `97` | `SI(RB)` | Silt over rubble |
| `98` | `SI(SD)` | Silt over sand |

### Fish Module

| Required field/planning item | Current design | Status | Recommended target |
| --- | --- | --- | --- |
| Fish recorder | `observations.observer_id` only | Partial | Add survey recorders by module role: benthos/fish/invert. |
| Choose whether body length is recorded separately | None | Missing | Add survey/module setting, or encode size buckets in fish metric catalog. |
| Per-species counts in four segments | Current UI enters one count per species for one date/location | Partial | Add `survey_taxa_counts`: `survey_id`, `segment_index`, `taxon_metric_id`, `count`. |
| Counts must be zero or positive integer | `observations.count >= 0` | Covered for simple observations | Keep validation for all count tables. |

Fish fields from the PDF should become fixed metric/catalog rows, not ad hoc species names:

| PDF field | Current mapping | Recommended target catalog key |
| --- | --- | --- |
| 蝶魚 | Generic `species` row possible | `butterflyfish` |
| 蝶魚 <5 | Generic `species` row possible | `butterflyfish_less5` |
| 石鱸 | Generic `species` row possible | `sweetlips` |
| 石鱸 juvenile | Generic `species` row possible | `sweetlips_juv` |
| 笛鯛 | Generic `species` row possible | `snapper` |
| 笛鯛 <20 | Generic `species` row possible | `snapper_less20` |
| 老鼠斑 | Generic `species` row possible | `barramundi_cod` |
| 蘇眉 | Generic `species` row possible | `humphead_wrasse` |
| 隆頭鸚哥 | Generic `species` row possible | `bumphead_parrotfish` |
| 鸚哥魚 | Generic `species` row possible | `other_parrotfish` |
| 鸚哥魚 <20 | Generic `species` row possible | `other_parrotfish_less20` |
| 裸胸鯙 | Generic `species` row possible | `moray_eel` |
| 石斑魚 <30 | Generic `species` row possible | `grouper_less30` |
| 石斑魚 30-40 | Generic `species` row possible | `grouper_30_40` |
| 石斑魚 40-50 | Generic `species` row possible | `grouper_40_50` |
| 石斑魚 50-60 | Generic `species` row possible | `grouper_50_60` |
| 石斑魚 >60 | Generic `species` row possible | `grouper_60` |
| 其他魚類 | Generic `species` row possible | `others` |

### Invertebrate Module

| Required field/planning item | Current design | Status | Recommended target |
| --- | --- | --- | --- |
| Invertebrate recorder | `observations.observer_id` only | Partial | Add survey recorders by module role. |
| Per-species counts in four segments | Current UI has one count per species | Partial | Reuse `survey_taxa_counts` with invertebrate metric catalog rows. |
| Counts must be zero or positive integer | `observations.count >= 0` | Covered for simple observations | Keep validation for all count tables. |

Invertebrate fields from the PDF:

| PDF field | Current mapping | Recommended target catalog key |
| --- | --- | --- |
| 珊瑚蝦 | Generic `species` row possible | `banded_coral_shrimp` |
| 魔鬼海膽 Diadema | Generic `species` row possible | `diadema` |
| 刺冠海膽 Echinothrix | Generic `species` row possible | `echinothrix` |
| 鉛筆海膽 | Generic `species` row possible | `pencil_urchin` |
| 收集海膽 | Generic `species` row possible | `collector_urchin` |
| 海參 | Generic `species` row possible | `seacucumber` |
| 棘冠海星 | Generic `species` row possible | `crown_of_thorns` |
| 法螺 | Generic `species` row possible | `triton` |
| 龍蝦 | Generic `species` row possible | `lobster` |
| 硨磲貝 <10 | Generic `species` row possible | `giantclam_less10` |
| 硨磲貝 10-20 | Generic `species` row possible | `giantclam_10_20` |
| 硨磲貝 20-30 | Generic `species` row possible | `giantclam_20_30` |
| 硨磲貝 30-40 | Generic `species` row possible | `giantclam_30_40` |
| 硨磲貝 40-50 | Generic `species` row possible | `giantclam_40_50` |
| 硨磲貝 >50 | Generic `species` row possible | `giantclam_50` |

### Environmental Impact Module

| Required field/planning item | Current design | Status | Recommended target |
| --- | --- | --- | --- |
| Per-impact counts in four segments | None | Missing | Add `survey_impact_counts`: `survey_id`, `segment_index`, `impact_type_id`, `count`. |
| Count validation: zero or positive integer | Only observation count has this | Partial | Apply to impact counts. |
| Grading rule: 0 none, 1 low, 2 medium, 3 high | None | Missing | Compute grade from count: `0`, `1`, `2-4`, `5+`. |
| Average/SD/SE after grading | None | Missing | Add report query/view/API. |

Impact fields from the PDF:

| PDF field | Recommended target key |
| --- | --- |
| 船錨 | `boat_anchor` |
| 炸魚 | `dynamite` |
| 其他珊瑚損害 | `other_coral_damage` |
| 漁網 | `fishnet` |
| 垃圾 | `trash` |
| 一般垃圾 | `general_trash` |
| 白化比例 population | `bleaching_population_percent` |
| 白化比例 colony | `bleaching_colony_percent` |
| 黑帶病 | `disease_coral_black_band` |
| 白帶病 | `disease_coral_white_band` |

### Rare Organisms And Comments

| Required field/planning item | Current design | Status | Recommended target |
| --- | --- | --- | --- |
| Sharks | Could be a `species` row | Partial | Add rare-organism metric rows or `survey_rare_organism_counts`. |
| Turtles | Could be a `species` row | Partial | Same as above. |
| Mantas | Could be a `species` row | Partial | Same as above. |
| Other rare organism | Could be notes | Partial | Store count plus free-text label/comment. |
| Comments | `observations.notes` | Partial | Add survey-level and module-level comments. |
| Coral damage other explanation | None | Missing | Add conditional required comment when `other_coral_damage > 0`. |

## Calculation Mapping

| Required output | Current design | Status | Recommended implementation |
| --- | --- | --- | --- |
| Per-substrate coverage rate | None | Missing | Count normalized substrate categories across 160 points: `SUM(category_total / 160) * 100`. |
| Per-sub-sample substrate coverage | None | Missing | Count each category in each 40-point segment: `segment_total / 40 * 100`. |
| Substrate SD | None | Missing | `STDEV.S` across the four segment coverage values. |
| Substrate SE | None | Missing | PDF formula: `SD / sqrt(4) / 40 * 100`; confirm formula once implementation starts because coverage values may already be percentages. |
| Live coral cover | None | Missing | `HC coverage + SC coverage`. |
| Fish average | Simple observation counts can be listed only | Missing | Average the four segment counts for each fish metric. |
| Fish SD/SE | None | Missing | Compute across the four segment counts. |
| Invertebrate average | Simple observation counts can be listed only | Missing | Average the four segment counts for each invertebrate metric. |
| Invertebrate SD/SE | None | Missing | Compute across the four segment counts. |
| Impact grade average/SD/SE | None | Missing | Convert raw counts to grades first, then compute average/SD/SE across four segments. |
| Historical data merge | None | Missing | Add reporting/export workflow keyed by site, depth, date, metric. |
| Historical chart typhoon marker | None | Missing | Add chart annotation data source. |

## Recommended Target Data Model

This is the smallest structure that maps cleanly to the PDF without overloading the current generic `observations` table.

```text
locations
- id
- chinese_name
- english_name

sites
- id
- location_id
- county
- chinese_name
- english_name
- latitude
- longitude

reef_check_surveys
- id
- survey_date
- site_id
- depth_m
- general_comments
- rkc_reason
- fish_length_mode
- created_by
- updated_by
- created_at
- updated_at

reef_check_survey_recorders
- survey_id
- role: benthos | fish | invertebrate
- user_id nullable
- recorder_name

reef_check_segments
- survey_id
- segment_index: 1..4
- label: 0-20m | 25-45m | 50-70m | 75-95m
- start_m
- end_m

substrate_codes
- code
- display_name
- normalized_category: HC | SC | RKC | NIA | SP | RC | RB | SD | SI | OT

substrate_points
- survey_id
- segment_index
- point_index: 1..40
- substrate_code

substrate_bleaching_counts
- survey_id
- segment_index
- hc_bleached_count
- sc_bleached_count

reef_check_metrics
- id
- module: fish | invertebrate | impact | rare_organism
- key
- chinese_name
- english_name
- size_class
- sort_order
- active

reef_check_metric_counts
- survey_id
- segment_index
- metric_id
- count
- comment
```

Design choice:

- Keep existing `observations` for the current generic observation workflow unless it is no longer needed.
- Add Reef Check-specific survey tables for the PDF workflow.
- Extend or replace the current `species` table only if the product decides all biodiversity catalogs should share one unified taxonomy. For the PDF, fixed metric rows are safer than free-form species rows because several fields are size classes or impact categories, not biological species.

## UI Mapping

| PDF workflow | Current UI | Required UI change |
| --- | --- | --- |
| Step 1 sample basic information | Observation page has date and location only | Add survey setup screen with date, county/location/site/depth/GPS and recorder roles. |
| Benthos: 4 sub-sample areas | None | Add benthos module with segment tabs or stepper. |
| Benthos: 40 code entries per segment | None | Add 40-cell grid with valid-code checking. |
| Benthos: 5-cell chunk, user presses `>>` | None | Add chunked input state and manual next control. |
| Benthos: HC/SC bleaching counts after 40 entries | None | Show HC/SC bleaching fields after segment completion. |
| RKC >= 10% reason prompt | None | Auto-calculate after all substrate points and require reason if threshold met. |
| Fish/invertebrate/impact: one row per metric, four segment columns | Current species count list has one count input per species | Replace with module tables: metric rows x four segment columns. |
| Rare organisms | None | Add optional rare-organism module. |
| Notes and coral damage other explanation | One notes textarea | Add survey-level comments and conditional module comments. |
| Historical charts | None | Add reporting screen after data model and calculation API exist. |

## Implementation Priority

1. Add the survey/site data model, because every other required field depends on a survey event.
2. Add Reef Check metric catalogs: substrate codes, fish metrics, invertebrate metrics, impact metrics, rare-organism metrics.
3. Add input storage for substrate points and per-segment metric counts.
4. Add calculation/reporting queries for coverage, averages, SD, SE, live coral cover, and impact grading.
5. Replace the current field entry screen with a Reef Check survey workflow, or keep both workflows if generic observations are still needed.
6. Add historical export/chart APIs and typhoon/event annotations.

## Bottom Line

The current design covers authentication, simple catalogs, and a generic count-based observation workflow. It does not yet model the Reef Check survey structure in the PDFs.

The main alignment decision is to introduce a new `reef_check_surveys` domain model instead of stretching the current `observations` table. The current `observations` table can remain useful for lightweight sightings, but it is not expressive enough for substrate point data, segment-based counts, module recorders, impact grading, or historical Reef Check reporting.
