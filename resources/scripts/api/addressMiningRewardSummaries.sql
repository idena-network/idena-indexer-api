SELECT mrs.epoch,
       mrs.amount,
       mrs.burnt
FROM mining_reward_summaries mrs
WHERE mrs.address_id = (SELECT id FROM addresses WHERE lower(address) = lower($1))
  AND ($3::bigint IS NULL OR mrs.epoch <= $3)
ORDER BY mrs.epoch DESC
LIMIT $2