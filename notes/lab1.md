mit 6.824 lab1

schedule https://pdos.csail.mit.edu/6.824/schedule.html
lab1  https://pdos.csail.mit.edu/6.824/labs/lab-mr.html
lec1 https://pdos.csail.mit.edu/6.824/notes/l01.txt
lec1论文  https://pdos.csail.mit.edu/6.824/papers/mapreduce.pdf
go tour faq https://pdos.csail.mit.edu/6.824/papers/tour-faq.txt




可以参考之前做的分布式任务调度
太tricky的实现是坏代码的征兆,杜绝

# 任务

worker通过rpc向master 获取任务

worker 从1或多个文件读取任务的input , 执行任务

把输出写入1个或多个文件




# 问题


## 如何定义Task



任务类型 1 map 2 reduce
文件名
任务状态
任务开始时间  ( 超时检查用) (master 需要设置超时时间 10秒, 如果worker没完成 , 把任务交给其他worker)

## 如何分配task
master维护一个未执行的map任务的记录表 , 一个执行中的 , 一个执行完毕的  x

再维护reduce任务的各种表


## worker如何执行任务
t
worker获取到task后 , 先去读取文件 , 然后执行对应task的函数 , 然后把输出写入到文件 , 

## 并”告知” master 任务完成了
worker把task和task状态,时间返回给master , master加锁 , master检查该task是否在运行 , 如果不在运行,直接返回 , 否则更新

是不是要等map任务都执行完才能执行reduce ?    是的! 看了测试用例了 , 所以当map全部执行完之后 , 才开始执行reduce任务

不能把map的中间输出放到内存里吗?  中间输出格式是?  map [key ]  intermidiate结构数组
The worker should put intermediate Map output in files in the current directory, where your worker can later read them as input to Reduce tasks.    
哈希放到相同的文件里 , 格式自己顶一个

mr-mid- hashkey



reduce的输出格式是什么  mr-out-*     
The worker implementation should put the output of the X'th reduce task in the file mr-out-X.
新加一个变量reduce计数


## 如何实现崩溃恢复? 崩溃指的是谁崩溃?  

这个先不做 , 开始写代码吧




go build -buildmode=plugin ../mrapps/wc.go

$ rm mr-out*
$ go run mrmaster.go pg-*.txt

$ go run mrworker.go wc.so   运行多个worker



需要实现mr里的文件 
先看一遍测试用例 main/test-mr.sh , 除了基本功能 , 还检查runs map and reduce tasks in parallel , crash recover


正确结果 :
$ cat mr-out-* | sort | more
A 509
ABOUT 2
ACT 8
...


$ sh ./test-mr.sh
*** Starting wc test.
--- wc test: PASS
*** Starting indexer test.
--- indexer test: PASS
*** Starting map parallelism test.
--- map parallelism test: PASS
*** Starting reduce parallelism test.
--- reduce parallelism test: PASS
*** Starting crash test.
--- crash test: PASS
*** PASSED ALL TESTS






* The master, as an RPC server, will be concurrent; don't forget to lock shared data.
* Use Go's race detector, with go build -race and go run -race. test-mr.sh has a comment that shows you how to enable the race detector for the tests.
* Workers will sometimes need to wait, e.g. reduces can't start until the last map has finished. One possibility is for workers to periodically ask the master for work, sleeping with time.Sleep() between each request. Another possibility is for the relevant RPC handler in the master to have a loop that waits, either with time.Sleep() or sync.Cond. Go runs the handler for each RPC in its own thread, so the fact that one handler is waiting won't prevent the master from processing other RPCs.
* The master can't reliably distinguish between crashed workers, workers that are alive but have stalled for some reason, and workers that are executing but too slowly to be useful. The best you can do is have the master wait for some amount of time, and then give up and re-issue the task to a different worker. For this lab, have the master wait for ten seconds; after that the master should assume the worker has died (of course, it might not have).
* To test crash recovery, you can use the mrapps/crash.go application plugin. It randomly exits in the Map and Reduce functions.
* To ensure that nobody observes partially written files in the presence of crashes, the MapReduce paper mentions the trick of using a temporary file and atomically renaming it once it is completely written. You can use ioutil.TempFile to create a temporary file and os.Rename to atomically rename it.
* test-mr.sh runs all the processes in the sub-directory mr-tmp, so if something goes wrong and you want to look at intermediate or output files, look there.




// crash test没通过

还剩下 任务超时重新分发

master另外开一个thread去每秒检查一次 , 如果任务超时 , 则把任务重新放到未执行队列中  x

worker在获取任务时要告知master自己的id , 并且master把id更新到对应task上  x

并且在worker写入中间文件之前 , 要先向master进行检查 , 如果发现任务已经被其他worker获取, 则不写入中间文件 x

worker的标识用什么?  pid ?

更新Task有可能有竞争条件 , 要加锁




worker崩溃的情况




想到的竞争问题

workerID的选型 , 进程id , 崩溃后会被重新分配给其他进程 , 可能有bug

task的workerid修改, 可能在刚好10秒的时候 , 完成写入了 又重新分配了  , 用状态模式 , master , 如果在执行定时检查就不能 check 和 updateTask  , 取什么名字好 ,  backGroudRunningBlock ?  算了先不加了 , 得重构了	




# 10s超时问题

每次worker准备map/reduce ouput的时候去master检查task是否已经executed , true则不写入, 进入下一次loop  x

master则定期检查Executing里是否有超时任务 , 有则重新分发 x

 还要检查是否已经执行完了 , 在周期任务里

task结构体是复制过去了 , 还是引用的同一个? 要测试一下





## crash test

* Workers will sometimes need to wait, e.g. reduces can't start until the last map has finished. One possibility is for workers to periodically ask the master for work, sleeping with time.Sleep() between each request. Another possibility is for the relevant RPC handler in the master to have a loop that waits, either with time.Sleep() or sync.Cond. Go runs the handler for each RPC in its own thread, so the fact that one handler is waiting won't prevent the master from processing other RPCs.
* The master can't reliably distinguish between crashed workers, workers that are alive but have stalled for some reason, and workers that are executing but too slowly to be useful. The best you can do is have the master wait for some amount of time, and then give up and re-issue the task to a different worker. For this lab, have the master wait for ten seconds; after that the master should assume the worker has died (of course, it might not have).
* To test crash recovery, you can use the mrapps/crash.go application plugin. It randomly exits in the Map and Reduce functions.
* To ensure that nobody observes partially written files in the presence of crashes, the MapReduce paper mentions the trick of using a temporary file and atomically renaming it once it is completely written. You can use ioutil.TempFile to create a temporary file and os.Rename to atomically rename it.


使用临时文件 , 完成的时候改名 , 避免看到崩溃产生的文件

worker进程可能需要等待，因为最后一个mapworker没有完成任务，为了避免忙等，可以设置time.sleep()或者条件变量
master需要掌握worker的工作时间，超过一定时间（lab建议10s）后重新调度任务
如果worker出现crash，应当确保它写入的临时文件不会被别的worker读取（因为是残缺文件），所以可以使用ioutil.TempFile创建临时文件并且使用os.Rename自动重命名
————————————————
版权声明：本文为CSDN博主「q3erf」的原创文章，遵循 CC 4.0 BY-SA 版权协议，转载请附上原文出处链接及本声明。
原文链接：https://blog.csdn.net/q3erf/java/article/details/105407446


最终输出文件可以很简单地重命名

但是中间文件多个task会map到同一个文件里 , 这个要怎么防止写入的时候crash ?





前面的设计有问题 , 没仔细看完论文 ,  现在为了fault tolerance要加的东西 , 改动还挺大的..

1 master 要维护一个worker的状态表 , 并定时去检查worker状态 

2 map worker 产生的中间文件只存在该worker的本地 , 所以reduce worker需要通过master得到对应hash值的map worker列表 , 向他们rpc请求获得数据 , 然后最终输出存到gfs



input is (already) split into M files
  Input1 -> Map -> a,1 b,1
  Input2 -> Map ->     b,1
  Input3 -> Map -> a,1     c,1
                    |   |   |
                    |   |   -> Reduce -> c,1
                    |   -----> Reduce -> b,2
                    ---------> Reduce -> a,2

3 一个worker可能执行了多个map task , 所以获取中间输出的时候还要带上hashkey

4 如果reduce worker去读取map workers的时候 , 某个map worker crash了咋办?

通知master重新跑 ? 比较麻烦 , 先不实现了


5 如果reduce worker崩溃了 , 重新跑就行了 , 用临时文件的策略

6 如果master 崩溃了?  需要 snapshot+append日志 , 用于恢复


7 哪些地方可能存在race条件? 
重新分发任务的地方 , 同时改task状态


8 重复执行任务了 , 是哪里?

任务还在执行中 , 写入文件的时候 ,  超时被重新分配 , 

解决: 在isTaskExecuted加锁 , 在Update完释放


master          worker1 									worker2

                       获取任务
			
		      执行任务        超时10秒

重新分配该任务										获取任务
													执行任务
                      											告知master任务完成
													写入文件

			告知master任务完成							

		     写入文件

告知这一步要加锁


# 做了哪些改进优化


# 后续

master failure

worker failure


问题 : 这rpc请求是可以多客户端并行的吗?  是的 , 所以需要加锁 , 后面再看底层 