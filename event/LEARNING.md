# event：进程内的订阅

[返回学习总览](../LEARNING.md) · 相关 [core](../core/LEARNING.md)、[eth](../eth/LEARNING.md)

`event` 在一个进程里把值发给多个 channel。链头切换、新日志、交易池更新都用它。跨节点的广播在 [p2p](../p2p/LEARNING.md) 和 [eth](../eth/LEARNING.md)。

## 建议阅读顺序

1. `event/subscription.go`：`Subscription`、`NewSubscription`、`Resubscribe`、`SubscriptionScope`。
2. `event/feed.go`：`Feed.Subscribe`、`Feed.Send`。
3. `event/feedof.go`：`FeedOf[T]`，类型由编译器固定。
4. `event/event.go`：`TypeMux`。注释写明已废弃，新代码用 `Feed`。
5. `event/example_feed_test.go`、`example_scope_test.go`：最短的用法。

## Feed

零值可以直接用。第一次 `Subscribe` 或 `Send` 决定元素类型，之后类型不一致会 panic。

`Subscribe` 收的是可发送的 channel。返回的 `Subscription` 有两件事：`Unsubscribe` 停止接收，`Err` 在订阅结束时被关闭。

`Send` 把同一个值写进当前所有订阅者。慢的订阅者不会被丢掉：它的 channel 满了，这次 `Send` 会一直等到能写进去，或者等到对方 `Unsubscribe`。所以订阅方的 channel 要有缓冲，否则一个慢读者会拖住整次发送。链头、日志这种热路径上尤其如此。

`FeedOf[T]` 做同样的事，channel 类型是 `chan<- T`，不会在运行时才发现类型错了。

## Subscription

`NewSubscription` 在后台跑一个 producer。producer 返回时，错误发给 `Err()`，订阅结束。调用方取消时，传给 producer 的 channel 被关闭。

`Resubscribe` 在内部订阅出错后按退避重新建立。RPC 的 websocket 订阅掉线重连用这个模式。`SubscriptionScope` 把一组订阅收在一起，`Close` 时全部 `Unsubscribe`。服务停机时用它，避免每个订阅单独记一份。

`JoinSubscriptions` 在 `multisub.go`，把多个订阅合成一个。调用方取消这个合成订阅，或其中一路返回非 nil 错误，会取消其余的。单独退订其中一路不会连带取消别的。

## 在节点里出现的地方

`core.BlockChain` 用 `Feed` 发出链头、日志和删块事件。`eth/filters` 订这些 feed，再通过 [rpc](../rpc/LEARNING.md) 推给 `eth_subscribe`。交易池也用 feed 通知新交易，[eth](../eth/LEARNING.md) 的 `txBroadcastLoop` 订的就是它。

## 读的时候记住

- 新代码用 `Feed` 或 `FeedOf`。`TypeMux.Post` 是旧接口。
- `Send` 不丢弃慢订阅者。缓冲要加在订阅方的 channel 上。
- 同一个 `Feed` 只能有一种元素类型。零值可用，类型在第一次调用时定下来。
- `Unsubscribe` 之后从 `Err()` 读到的是 channel 关闭。
