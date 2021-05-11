SELECT upgrade, start_activation_date, end_activation_date
FROM upgrades
WHERE $2::bigint IS NULL
   OR upgrade <= $2::bigint
ORDER BY upgrade DESC
LIMIT $1