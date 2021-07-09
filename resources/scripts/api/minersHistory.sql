SELECT mh.block_timestamp, mh.online_miners, mh.online_validators
FROM miners_history mh
ORDER BY mh.block_timestamp DESC
LIMIT 2000