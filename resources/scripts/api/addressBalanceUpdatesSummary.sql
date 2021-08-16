SELECT balance_in, balance_out, stake_in, stake_out, penalty_in, penalty_out
FROM balance_update_summaries
WHERE address_id = (SELECT id FROM addresses WHERE lower(address) = lower($1))