
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
    - [x] TestBasicAgree2B
    - [x] TestRPCBytes2B
    - [x] TestFailAgree2B 
    - [x] TestFailNoAgree2B 
    - [x] TestConcurrentStarts2B 
    - [x] TestRejoin2B 
    - [x] TestBackup2B 
    - [x] TestCount2B

  - [x] Part 2C
    - [x] Some2cPersistSimpleTest
    - [x] TestFigure82C
    - [x] TestUnreliableAgree2C
    - [x] TestFigure8Unreliable2C *****
    - [x] TestUnreliableChurn2C *****
    - [x] TestReliableChurn2C


- [ ] Lab3 KV Raft
  - [x] Part 3A KV Client/Server
    - [x] TestBasic3A
    - [x] TestConcurrent3A
    - [x] TestUnreliable3A
    - [x] TestUnreliableOneKey3A
    - [x] TestOnePartition3A
    - [x] TestManyPartitionsOneClient3A
    - [x] TestManyPartitionsManyClients3A
    - [x] TestPersistOneClient3A
    - [x] TestPersistConcurrent3A
    - [x] TestPersistConcurrentUnreliable3A
    - [x] TestPersistPartition3A
    - [x] TestPersistPartitionUnreliable3A
    - [x] TestPersistPartitionUnreliableLinearizable3A
  - [ ] Part 3B Log Compaction
    - [ ] TestSnapshotSize3B
    - [ ] TestSnapshotRecover3B
    - [ ] TestSnapshotRecoverManyClients3B
    - [ ] TestSnapshotUnreliable3B
    - [ ] TestSnapshotUnreliableRecover3B
    - [ ] TestSnapshotUnreliableRecoverConcurrentPartition3B
    - [ ] TestSnapshotUnreliableRecoverConcurrentPartitionLinearizable3B
  
- [ ] Lab4 Sharded KV
  - [ ] Part 4A
  - [ ] Part 4B


---

### Lab3A

 2020/07/03 - 2020/08/01

> bug 2 concurrent map read and write

3A 遇到了很多并发 bug,总结了一下主要原因,没有梳理好哪些地方会产生并发

1.同一个客户端会同时发出两个请求吗(get 和 putappend 的排列组合) , 如果会,生成 reqId 哪里有 bug

2.同一个 server 会同时处理多个 client 的请求 吗 , 会

3.会不会同时处理一个 client 的多个请求(先梳理 1)


加锁的定义要理清楚,到底是在对什么操作,什么资源加锁 , 是想让哪些逻辑串行执行

写基本都发生在 loop thread 中, 写的时候, 上层的 get put append 不能读, 暂时先这样

从画的图来看的话,就是 server1 的多个请求线程和 loop thread 之间需要保证线程安全


> bug 1 OnePartition3A

分区故障恢复后, index 对应的 op 可能不是同一个(被新 leader 的覆盖),但是 channel 已经关联了 index 了,有什么其他办法区分这种场景呢? op 的一致性检查,并让客户端重试 ,还好 raft 认真做了,细节这么久还记得


section 8 翻译摘抄
> https://www.cnblogs.com/linbingdong/p/6442673.html
8 客户端交互
本节介绍客户端如何和 Raft 进行交互，包括客户端如何找到 leader 和 Raft 是如何支持线性化语义的。这些问题对于所有基于一致性的系统都存在，并且 Raft 的解决方案和其他的也差不多。

> Raft 的客户端发送所有的请求给 leader 。当客户端第一次启动的时候，它会随机挑选一个服务器进行通信。如果客户端第一次挑选的服务器不是 leader ，那么该服务器会拒绝客户端的请求并且提供关于它最近接收到的领导人的信息（AppendEntries 请求包含了 leader 的网络地址）。如果 leader 已经崩溃了，客户端请求就会超时；客户端之后会再次随机挑选服务器进行重试。

> 我们 Raft 的目标是要实现线性化语义（每一次操作立即执行，只执行一次，在它的调用和回复之间）。但是，如上述，Raft 可能执行同一条命令多次：例如，如果 leader 在提交了该日志条目之后，响应客户端之前崩溃了，那么客户端会和新的 leader 重试这条指令，导致这条命令被再次执行。解决方案就是客户端对于每一条指令都赋予一个唯一的序列号。然后，状态机跟踪每个客户端已经处理的最新的序列号以及相关联的回复。如果接收到一条指令，该指令的序列号已经被执行过了，就立即返回结果，而不重新执行该请求。

> 只读的操作可以直接处理而不需要记录日志。但是，如果不采取任何其他措施，这么做可能会有返回过时数据（stale data）的风险，因为 leader 响应客户端请求时可能已经被新的 leader 替代了，但是它还不知道自己已经不是最新的 leader 了。线性化的读操作肯定不会返回过时数据，Raft 需要使用两个额外的预防措施来在不使用日志的情况下保证这一点。首先，leader 必须有关于哪些日志条目被提交了的最新信息。Leader 完整性特性保证了 leader 一定拥有所有已经被提交的日志条目，但是在它任期开始的时候，它可能不知道哪些是已经被提交的。为了知道这些信息，它需要在它的任期里提交一个日志条目。Raft 通过让 leader 在任期开始的时候提交一个空的没有任何操作的日志条目到日志中来处理该问题。第二，leader 在处理只读请求之前必须检查自己是否已经被替代了（如果一个更新的 leader 被选举出来了，它的信息就是过时的了）。Raft 通过让 leader 在响应只读请求之前，先和集群中的过半节点交换一次心跳信息来处理该问题。另一种可选的方案，leader 可以依赖心跳机制来实现一种租约的形式，但是这种方法依赖 timing 来保证安全性（假设时间误差是有界的）。



```

2020/7/27 画了图和伪代码,梳理一下

+-------------------+              set A ,1                                                                   +-------------------------+
|                   | +-------------------------------------->                                                |                         |
|                   |                                           +----------------------+       chanArr[index] |                         |
|   Client          |                                           |                      |  <-------------------+     loop thread         |
|                   |                                           |                      |                      |                         |
|                   |                                           |       Server 1       |                      +-------------------------+
+-------------------+           set B ,1                        |                      |                                        ^+
                       +------------------------------------>   |                      |                                         |
                                                                +-----+----------------+                                         |
                                                                      |                                                          |
                                                                      |                                                          |
                                                                      |                                                          |
                                                                      |start and generate                                        |
                                                                      |chan assoc with index                                     |
                                                                      |                                                          |
                                                                      |                                                          |
                                                                      |                                                          |
                                                                      v                                                          |
                                                               +------+----------+                                               |
                                                               |                 ++                         applyCh              |
                                                               |                 +-----------------------------------------------+
                                                               |     Raft1       |
                                                               |                 |
                                                               +-----------------+

```

我的实现:
```

[get] ->  [duplicate dectect]
  
  ^       [raft leader] -> [follower]
                          [follower]
 applyThread <-
 


```

```
[get] ->
            [raft leader] -> [follower]
[get] ->                     [follower]


applyThread 和每个 op 之间建立一个管道

可以同时复制到大多数 follower 提高并发 
减少了上一个实现单 op 加锁造成的等待

```

lab3 自己是用状态机的模型写的, 虽然能过但是在 PutAppend 和 Get 都加了锁,并发很低 . 去学习了别人的实现 , 发现设计思路很棒 , 模仿了 raft 的 start()函数和 applyCh , 对每个 op 进行 start , 然后每个 op 分配一个 channel 等待结果 , 并发无敌 , 感叹自己实在是想不到这种写法 , (队列+异步通知模型)


### Lab2

2020/06/04 ~ 2020/06/07

分支在`rewrite_lab2_0604`

为了解决TestFigure8Unreliable2C跑 100 次不能稳定通过,以至于不敢做 lab3,隔了一个月的时间,开始了 lab2 第三次重写,用回了第一次写时候的思路,event-driven,只不过这次把需要并发和不能并发的逻辑理清楚了,每个 peer 有 3 个线程,一个选举超时线程,一个心跳线程,一个主事件线程(主要是心跳,投票请求和响应的处理)

说一下需要并发的逻辑: 群发心跳包,群发选票 , 但是发送之前的参数准备是不能并发的,需要加锁(和主事件线程互斥)
不能并发的逻辑:  除开并发逻辑之外的基本都是 , 主要有心跳,选票请求和响应, 选举超时事件 , 客户端发起的agreement

然后是TestFigure8Unreliable2C这个 case, 必须优化日志复制的逻辑,否则跑 100 次的通过率会很低,有 commited 超时限制 ,  需要实现 fast backup (快速回退): 当 follower 发现 prevLogTerm 不一致的时候,会在本机的 log 中找到该 term 第一个 log 的 index, 发给 leader . 如果没有 , 那么 leader会把该 follower 的 nextIndex 置为 leader 的 log 中该 term 的第一个 index , 实现快速回退一个任期的 log , 而不是一个一个回退

总算没有烂尾..........


### Lab1

Lab1 用了 3 天时间,没什么难度,就不做总结了… 不像 Lab2 一个 bug 就是3天



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


