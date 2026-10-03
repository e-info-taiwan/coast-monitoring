-- Aggregate rows are always derived from the declared members, including NA.
-- Historical hand-entered aggregate observations remain raw but are not used.
CREATE OR REPLACE VIEW belt_summary AS
 WITH observations AS (
 SELECT o.transect_id,o.taxon_id,o.segment,CASE WHEN o.record_status='recorded' THEN o.count END AS value
 FROM belt_observation o JOIN taxon x ON x.id=o.taxon_id WHERE NOT x.is_aggregate
 UNION ALL
 SELECT o.transect_id,agg.id,o.segment,
 CASE WHEN count(*)=count(*) FILTER (WHERE o.record_status='recorded')
 AND count(DISTINCT member.id)=(SELECT count(*) FROM taxon m WHERE m.aggregate_of=agg.aggregate_of AND m.taxon_group=agg.taxon_group AND NOT m.is_aggregate AND m.is_active)
 THEN sum(o.count)::int END
 FROM taxon agg JOIN taxon member ON member.aggregate_of=agg.aggregate_of AND member.taxon_group=agg.taxon_group AND NOT member.is_aggregate AND member.is_active
 JOIN belt_observation o ON o.taxon_id=member.id WHERE agg.is_aggregate AND agg.is_active GROUP BY o.transect_id,agg.id,o.segment,agg.aggregate_of,agg.taxon_group
 ) SELECT transect_id,taxon_id,count(value)::int AS n,sum(value) AS total,avg(value) AS mean,stddev_samp(value) AS sd,
 stddev_samp(value)/sqrt(NULLIF(count(value),0)) AS se FROM observations GROUP BY transect_id,taxon_id;
