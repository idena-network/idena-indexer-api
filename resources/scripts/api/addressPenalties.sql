select p.id,
       a.address,
       p.penalty,
       p.block_height,
       b.hash,
       b.timestamp,
       b.epoch
from penalties p
         join blocks b on b.height = p.block_height
         join addresses a on a.id = p.address_id and lower(a.address) = lower($1)
WHERE $3::bigint IS NULL
   OR p.id <= $3
order by p.id desc
limit $2