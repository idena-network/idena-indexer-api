select count(*)
from flips f
         join transactions t on t.id = f.tx_id
         JOIN flip_pics fp ON fp.index = 3 AND fp.fd_flip_tx_id = f.tx_id
         join blocks b on b.epoch = $1 and b.height = t.block_height