SELECT p.address_id, a.address, p.size
FROM pool_sizes p
         JOIN addresses a ON a.id = p.address_id
WHERE $2::bigint IS NULL
   OR p.size = $3 AND p.address_id >= $2
   OR p.size < $3
ORDER BY p.size DESC, p.address_id
LIMIT $1