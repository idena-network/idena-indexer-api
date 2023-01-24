SELECT invb.epoch,
       invt.hash,
       inva.address                             inviter,
       coalesce(rsa.amount, 0)                  inviter_stake,
       coalesce(invdics.name, 'Undefined')      inviter_state,
       coalesce(invitee_dics.name, 'Undefined') state
FROM latest_activation_txs lat
         LEFT JOIN activation_txs act ON act.tx_id = lat.activation_tx_id
         LEFT JOIN transactions invt ON invt.id = act.invite_tx_id
         LEFT JOIN blocks invb ON invb.height = invt.block_height
         LEFT JOIN addresses inva ON inva.id = invt.from
         LEFT JOIN epoch_identities invei ON invei.address_id = invt.from AND invei.epoch = $1
         LEFT JOIN reward_staked_amounts rsa ON rsa.ei_address_state_id = invei.address_state_id
         LEFT JOIN address_states invs ON invs.id = invei.address_state_id
         LEFT JOIN dic_identity_states invdics ON invdics.id = invs.state
         LEFT JOIN epoch_identities invitee_ei ON invitee_ei.address_id = lat.address_id AND invitee_ei.epoch = $1
         LEFT JOIN address_states invitee_s ON invitee_s.id = invitee_ei.address_state_id
         LEFT JOIN dic_identity_states invitee_dics ON invitee_dics.id = invitee_s.state
WHERE lat.epoch <= $1
  AND lat.epoch >= $1 - 2
  AND lat.address_id = (SELECT id FROM addresses WHERE lower(address) = lower($2))
ORDER BY activation_tx_id DESC
LIMIT 1;