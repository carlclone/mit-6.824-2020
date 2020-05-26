
All labs and assignment for the course

- [x] Lab1 MapReduce
  - [x] WordCount test 
  - [x] Indexer test 
  - [x] Map parallelism test 
  - [x] Reduce parallelism test 
  - [x] Task timeout redistribute test 
  - [x] Crash Test 
  
- [x] Lab2 Raft
  - [x] Part 2A 
    - [x] Initial Election 
    - [x] ReElection with network failure 


  - [x] Part 2B
    - [x] TestBasicAgree2B 最理想情况下的客户端请求 ,日志复制
    - [x] TestRPCBytes2B 保证一次agreement 每个 peer只发送一次 RPC
    - [x] TestFailAgree2B 
    - [x] TestFailNoAgree2B 
    - [x] TestConcurrentStarts2B 
    - [x] TestRejoin2B 
    - [x] TestBackup2B 
    - [x] TestCount2B 优化,减少一次选举需要的 RPC 次数

  - [x] Part 2C
    - [x] Some2cPersistSimpleTest
    - [x] TestFigure82C
    - [x] TestUnreliableAgree2C
    - [x] TestFigure8Unreliable2C *****
    - [x] TestUnreliableChurn2C *****
    - [x] TestReliableChurn2C


- [ ] Lab3 KV Raft
- [ ] Lab4 Sharded KV


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


### 资料

[课表](https://pdos.csail.mit.edu/6.824/schedule.html)


某个场景的paper
https://conferences.sigcomm.org/sigcomm/2015/pdf/papers/p85.pdf

 
## 思考记录

加锁意义不明 每一段代码都要能说出为什么存在 意义是什么

加锁的粒度可以很好的参考acid的3个情况 write write. Read write  write read 看你是要避免什么问题来决定锁的粒度 比如两个线程加一 write write问题

并发编程的设计不能直接把语言转换过来 会出现糟糕的设计 实际上任何都是 最好定义好状态机 ，要用上一切最好的流程工具了 画图定义伪代码