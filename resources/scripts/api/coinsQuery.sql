SELECT c.total_balance,
       c.total_stake,
       cs.total_burnt,
       cs.total_minted
FROM coins c,
     coins_summary cs
ORDER BY c.block_height DESC
LIMIT 1;