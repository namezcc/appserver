package module

import (
	"context"
	"sync"
	"time"
)

type redisCall func(context.Context)

func doRedis(_f redisCall) {
	ctx, cancle := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancle()
	_f(ctx)
}

func lockFunc(_l *sync.Mutex, _f func()) {
	_l.Lock()
	defer _l.Unlock()
	_f()
}

func redisFunc(m httpBase, _f redisCall) {
	l := m.getRedisLock()
	l.Lock()
	defer l.Unlock()
	ctx, cancle := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancle()
	_f(ctx)
}
