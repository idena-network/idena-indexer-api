select e.epoch,
       e.validation_time,
       coalesce(es.flip_lottery_block_height, 0) flip_lottery_block_height,
       coalesce(preves.min_score_for_invite, 0)      min_score_for_invite
from epochs e
         left join epoch_summaries preves on preves.epoch = e.epoch - 1
         left join epoch_summaries es on es.epoch = e.epoch
where e.epoch = $1