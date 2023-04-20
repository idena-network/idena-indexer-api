SELECT dh.delegation_tx_id,
       delegateea.address       delegatee_address,
       delegationt.hash         delegation_tx_hash,
       delegationtb.timestamp   delegation_tx_timestamp,
       delegationb.height       delegation_block_height,
       delegationb.hash         delegation_block_hash,
       delegationb.epoch        delegation_block_epoch,
       delegationb.timestamp    delegation_block_timestamp,
       undelegationt.hash       undelegation_tx_hash,
       undelegationtt.name      undelegation_tx_type,
       undelegationtb.timestamp undelegation_tx_timestamp,
       undelegationb.height     undelegation_block_height,
       undelegationb.hash       undelegation_block_hash,
       undelegationb.epoch      undelegation_block_epoch,
       undelegationb.timestamp  undelegation_block_timestamp,
       dicr.name                undelegation_reason
FROM delegation_history dh
         LEFT JOIN transactions delegationt ON delegationt.id = dh.delegation_tx_id
         LEFT JOIN addresses delegateea ON delegateea.id = delegationt.to
         LEFT JOIN blocks delegationtb ON delegationtb.height = delegationt.block_height
         LEFT JOIN blocks delegationb ON delegationb.height = dh.delegation_block_height
         LEFT JOIN transactions undelegationt ON undelegationt.id = dh.undelegation_tx_id
         LEFT JOIN dic_tx_types undelegationtt ON undelegationtt.id = undelegationt.type
         LEFT JOIN blocks undelegationtb ON undelegationtb.height = undelegationt.block_height
         LEFT JOIN blocks undelegationb ON undelegationb.height = dh.undelegation_block_height
         LEFT JOIN dic_undelegation_reasons dicr ON dicr.id = dh.undelegation_reason
WHERE dh.delegator_address_id = (SELECT id FROM addresses WHERE lower(address) = lower($1))
  AND ($3::bigint IS NULL OR dh.delegation_tx_id <= $3)
ORDER BY dh.delegation_tx_id DESC
LIMIT $2
