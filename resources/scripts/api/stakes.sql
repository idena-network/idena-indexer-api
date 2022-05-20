SELECT b.address_id, a.address, b.balance, b.stake
FROM balances b
         LEFT JOIN addresses a ON a.id = b.address_id
WHERE $2::bigint IS NULL
   OR b.stake = $3 AND b.address_id >= $2
   OR b.stake < $3
ORDER BY b.stake DESC, b.address_id
LIMIT $1