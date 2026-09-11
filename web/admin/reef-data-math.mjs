// Summaries use only recorded values. A missing segment is never an observed zero.
export function statistics(values) {
  const present = values.filter(v => v !== null && v !== undefined)
  const n = present.length
  if (!n) return { n, total: null, mean: null, sd: null }
  const total = present.reduce((a, b) => a + b, 0)
  const mean = total / n
  const sd = n > 1 ? Math.sqrt(present.reduce((sum, v) => sum + (v - mean) ** 2, 0) / (n - 1)) : null
  return { n, total, mean, sd }
}

export function impactValue(row) {
  return row.has_raw_count ? Math.min(row.raw_value, 3) : row.raw_value
}

export function substrateSummary(points, layer = "surface") {
  const selected = points.filter(p => p.substrate_layer === layer)
  const valid = selected.filter(p => p.substrate_code !== "NA")
  const codes = [...new Set(valid.map(p => p.substrate_code))].sort()
  return {
    recorded: selected.length,
    unknown: selected.length - valid.length,
    valid: valid.length,
    rows: codes.map(code => {
      const counts = [1, 2, 3, 4].map(segment => {
        const segmentPoints = valid.filter(p => p.segment === segment)
        return segmentPoints.length ? segmentPoints.filter(p => p.substrate_code === code).length : null
      })
      const stats = statistics(counts)
      return { code, counts, ...stats, cover: valid.length ? stats.total / valid.length * 100 : null }
    }),
  }
}

export function bleachingPercent(points, segment, code, count) {
  const n = points.filter(p => p.substrate_layer === "surface" && p.segment === segment && p.substrate_code === code).length
  return n ? count / n * 100 : null
}
