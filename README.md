
All labs and assignment for the course

## Lab1 MapReduce

WordCount test √

Indexer test √

Map parallelism test √

Reduce parallelism test √

Task timeout redistribute test √

Crash Test √

# Lab2 Raft

## Part 2A √

Initial Election √

ReElection with network failure √ 

## Part 2B

TestBasicAgree2B √
最理想情况下的客户端请求 ,日志复制

TestRPCBytes2B √
保证一次agreement 每个 peer只发送一次 RPC

TestFailAgree2B √ 

TestFailNoAgree2B √

TestConcurrentStarts2B √

TestRejoin2B √

TestBackup2B √

TestCount2B 优化,减少一次选举需要的 RPC 次数







---


### FailAgree 和 FailNoAgree 的场景

摘抄自https://pdos.csail.mit.edu/6.824/notes/l-raft2.txt

```
how can logs disagree after a crash?
  a leader crashes before sending last AppendEntries to all
    S1: 3
    S2: 3 3
    S3: 3 3
  worse: logs might have different commands in same entry!
    after a series of leader crashes, e.g.
        10 11 12 13  <- log entry #
    S1:  3
    S2:  3  3  4
    S3:  3  3  5

Raft forces agreement by having followers adopt new leader's log
  example:
  S3 is chosen as new leader for term 6
  S3 sends an AppendEntries with entry 13
     prevLogIndex=12
     prevLogTerm=5
  S2 replies false (AppendEntries step 2)
  S3 decrements nextIndex[S2] to 12
  S3 sends AppendEntries w/ entries 12+13, prevLogIndex=11, prevLogTerm=3
  S2 deletes its entry 12 (AppendEntries step 3)
  similar story for S1, but S3 has to back up one farther
  ```
