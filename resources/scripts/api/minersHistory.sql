SELECT coalesce(b.timestamp, 0) "timestamp", mh.online_miners, mh.online_validators
FROM miners_history mh
         LEFT JOIN blocks b on b.height = mh.block_height
ORDER BY block_height DESC
LIMIT 500