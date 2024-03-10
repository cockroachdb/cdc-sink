WITH data_0 AS (
SELECT nanos, logical, key, mut, before
FROM "_cdc_sink"."public"."my_db_public_tbl0"
WHERE (nanos, logical, key) > (($1::INT8[])[1], ($2::INT8[])[1], ($5::STRING[])[1])
AND (nanos, logical) < ($3, $4)
AND NOT applied
ORDER BY nanos, logical, key
),
data_1 AS (
SELECT nanos, logical, key, mut, before
FROM "_cdc_sink"."public"."my_db_public_tbl1"
WHERE (nanos, logical, key) > (($1::INT8[])[2], ($2::INT8[])[2], ($5::STRING[])[2])
AND (nanos, logical) < ($3, $4)
AND NOT applied
ORDER BY nanos, logical, key
),
data_2 AS (
SELECT nanos, logical, key, mut, before
FROM "_cdc_sink"."public"."my_db_public_tbl2"
WHERE (nanos, logical, key) > (($1::INT8[])[3], ($2::INT8[])[3], ($5::STRING[])[3])
AND (nanos, logical) < ($3, $4)
AND NOT applied
ORDER BY nanos, logical, key
),
data_3 AS (
SELECT nanos, logical, key, mut, before
FROM "_cdc_sink"."public"."my_db_public_tbl3"
WHERE (nanos, logical, key) > (($1::INT8[])[4], ($2::INT8[])[4], ($5::STRING[])[4])
AND (nanos, logical) < ($3, $4)
AND NOT applied
ORDER BY nanos, logical, key
)
SELECT * FROM (
SELECT 0 idx, nanos, logical, key, mut, before FROM data_0 UNION ALL
SELECT 1 idx, nanos, logical, key, mut, before FROM data_1 UNION ALL
SELECT 2 idx, nanos, logical, key, mut, before FROM data_2 UNION ALL
SELECT 3 idx, nanos, logical, key, mut, before FROM data_3)
ORDER BY nanos, logical, idx, key
LIMIT 10000