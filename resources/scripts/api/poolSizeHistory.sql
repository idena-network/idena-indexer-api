SELECT psh.epoch,
       coalesce(prev_psh.end_size, 0) start_size,
       psh.validation_size,
       psh.end_size
FROM pool_size_history psh
         LEFT JOIN pool_size_history prev_psh
                   ON prev_psh.epoch = psh.epoch - 1 AND prev_psh.address_id = psh.address_id
WHERE psh.address_id = (SELECT id FROM addresses WHERE lower(address) = lower($1))
  AND ($3::bigint IS NULL OR psh.epoch <= $3)
  AND psh.validation_size > 0
  AND coalesce(prev_psh.end_size, 0) > 0
ORDER BY psh.epoch DESC
LIMIT $2