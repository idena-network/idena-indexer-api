WITH epoch_tx_bounds AS (SELECT min_tx_id, max_tx_id FROM epoch_summaries WHERE epoch = $1)
SELECT f.tx_id,
       f.Cid,
       f.Size,
       a.address                         author,
       b.epoch,
       COALESCE(dfs.name, '')            status,
       COALESCE(da.name, '')             answer,
       coalesce(f.grade, 0) = 1          reported,
       coalesce(fs.wrong_words_votes, 0) wrong_words_votes,
       coalesce(fs.short_answers, 0)     short_answers,
       coalesce(fs.long_answers, 0)      long_answers,
       b.timestamp,
       coalesce(fi."data", ''::bytea)    icon,
       coalesce(fw.word_1, 0)            word_id_1,
       coalesce(wd1.name, '')            word_name_1,
       coalesce(wd1.description, '')     word_desc_1,
       coalesce(fw.word_2, 0)            word_id_2,
       coalesce(wd2.name, '')            word_name_2,
       coalesce(wd2.description, '')     word_desc_2,
       coalesce(fs.encrypted, false)     with_private_part,
       coalesce(f.grade, 0)              grade,
       coalesce(f.grade_score, 0)        grade_score
FROM flips f
         LEFT JOIN transactions t ON t.id = f.tx_id
         LEFT JOIN addresses a ON a.id = t.from
         LEFT JOIN blocks b ON b.height = t.block_height
         LEFT JOIN flip_icons fi ON fi.fd_flip_tx_id = f.tx_id
         LEFT JOIN flip_words fw ON fw.flip_tx_id = f.tx_id
         LEFT JOIN words_dictionary wd1 ON wd1.id = fw.word_1
         LEFT JOIN words_dictionary wd2 ON wd2.id = fw.word_2
         LEFT JOIN dic_flip_statuses dfs ON dfs.id = f.status
         LEFT JOIN dic_answers da ON da.id = f.answer
         LEFT JOIN flip_summaries fs ON fs.flip_tx_id = f.tx_id
WHERE (f.tx_id BETWEEN (SELECT min_tx_id FROM epoch_tx_bounds) AND (SELECT max_tx_id FROM epoch_tx_bounds))
  AND ($3::bigint IS NULL OR f.tx_id <= $3)
  AND f.delete_tx_id IS NULL
ORDER BY f.tx_id DESC
LIMIT $2;