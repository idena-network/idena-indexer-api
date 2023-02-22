SELECT ''                                     cid,
       ad.address,
       ida.name                               resp_answer,
       a.grade = 1                            resp_reported,
       coalesce(fda.name, '')                 flip_answer,
       coalesce(f.grade, 0) = 1               flip_reported,
       coalesce(dfs.name, '')                 status,
       a.point,
       a.grade                                resp_grade,
       coalesce(f.grade, 0)                   flip_grade,
       coalesce(a.index, 0)                   "index",
       coalesce(a.considered, true)           considered,
       coalesce(ei.wrong_grade_reason, 0) > 0 grade_ignored
FROM answers a
         JOIN flips f ON f.tx_id = a.flip_tx_id AND lower(f.cid) = lower($1)
         LEFT JOIN epoch_identities ei ON ei.address_state_id = a.ei_address_state_id
         LEFT JOIN addresses ad ON ad.id = ei.address_id
         LEFT JOIN dic_flip_statuses dfs ON dfs.id = f.status
         LEFT JOIN dic_answers fda ON fda.id = f.answer
         LEFT JOIN dic_answers ida ON ida.id = a.answer
WHERE a.is_short = $2