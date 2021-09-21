SELECT tr.epoch,
       tr.total,
       tr.validation,
       tr.flips,
       tr.invitations,
       tr.foundation,
       tr.zero_wallet,
       tr.validation_share,
       tr.flips_share,
       tr.invitations_share,
       es.block_count as epoch_duration
FROM total_rewards tr
         LEFT JOIN epoch_summaries es
                   ON es.epoch = tr.epoch
WHERE tr.epoch = $1