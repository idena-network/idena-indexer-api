select tr.epoch,
       tr.total,
       tr.validation,
       tr.flips,
       tr.invitations,
       tr.foundation,
       tr.zero_wallet,
       tr.validation_share,
       tr.flips_share,
       tr.invitations_share
from total_rewards tr
where tr.epoch = $1