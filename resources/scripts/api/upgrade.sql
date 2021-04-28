SELECT start_activation_date, end_activation_date
FROM upgrades
WHERE upgrade = $1