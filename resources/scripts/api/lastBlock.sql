SELECT b.epoch,
       b.height,
       b.hash,
       b.timestamp,
       (SELECT count(*) FROM transactions WHERE block_height = b.height)         tx_count,
       b.validators_count,
       coalesce(pa.address, '')                                                  proposer,
       coalesce(vs.vrf_score, 0)                                                 proposer_vrf_score,
       b.is_empty,
       b.body_size,
       b.full_size,
       b.vrf_proposer_threshold,
       b.fee_rate,
       (SELECT array_agg("flag") FROM block_flags WHERE block_height = b.height) flags,
       b.upgrade
FROM blocks b
         LEFT JOIN block_proposers p on p.block_height = b.height
         LEFT JOIN block_proposer_vrf_scores vs on vs.block_height = b.height
         LEFT JOIN addresses pa on pa.id = p.address_id
ORDER BY b.height DESC
LIMIT 1