INSERT INTO "database"."schema"."table" (
"pk0","pk1","val0","val1","enum","has_default"
) VALUES
($1::STRING,($2+$2)::INT8,('fixed')::STRING,($3||'foobar')::STRING,$4::"database"."schema"."MyEnum",CASE WHEN $5::INT = 1 THEN $6::INT8 ELSE expr() END),
($7::STRING,($8+$8)::INT8,('fixed')::STRING,($9||'foobar')::STRING,$10::"database"."schema"."MyEnum",CASE WHEN $11::INT = 1 THEN $12::INT8 ELSE expr() END)
ON CONFLICT ( "pk0","pk1" )
DO UPDATE SET ("val0","val1","enum","has_default") = ROW(excluded."val0",excluded."val1",excluded."enum",excluded."has_default")