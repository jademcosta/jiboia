# Resuse accumulator slice

The accumulator slice initially was recreated everytime a flush occured. I did a benchmark and found that if we reuse it we could have a big improvement on the allocations. Below is the benchstat result of the 2 solutions ("old" is the recreate one and "new" is the one that reuses):

```
name                                   old time/op    new time/op    delta
AccumulatorAppend/input_size_100-4       1.33ms ± 0%    0.78ms ± 0%   ~     (p=1.000 n=1+1)
AccumulatorAppend/input_size_1000-4      4.33ms ± 0%    3.72ms ± 0%   ~     (p=1.000 n=1+1)
AccumulatorAppend/input_size_74382-4      255ms ± 0%     257ms ± 0%   ~     (p=1.000 n=1+1)
AccumulatorAppend/input_size_382399-4     1.32s ± 0%     1.26s ± 0%   ~     (p=1.000 n=1+1)

name                                   old alloc/op   new alloc/op   delta
AccumulatorAppend/input_size_100-4       3.08MB ± 0%    2.26MB ± 0%   ~     (p=1.000 n=1+1)
AccumulatorAppend/input_size_1000-4      21.6MB ± 0%    20.8MB ± 0%   ~     (p=1.000 n=1+1)
AccumulatorAppend/input_size_74382-4     1.57GB ± 0%    1.57GB ± 0%   ~     (p=1.000 n=1+1)
AccumulatorAppend/input_size_382399-4    7.68GB ± 0%    7.68GB ± 0%   ~     (p=1.000 n=1+1)

name                                   old allocs/op  new allocs/op  delta
AccumulatorAppend/input_size_100-4        16.7k ± 0%     11.1k ± 0%   ~     (p=1.000 n=1+1)
AccumulatorAppend/input_size_1000-4       16.7k ± 0%     11.1k ± 0%   ~     (p=1.000 n=1+1)
AccumulatorAppend/input_size_74382-4      16.7k ± 0%     11.1k ± 0%   ~     (p=1.000 n=1+1)
AccumulatorAppend/input_size_382399-4     16.7k ± 0%     11.2k ± 0%   ~     (p=1.000 n=1+1)
```

(`input_size` is the size (in bytes) of the input sent to be accumulated, while the desired size was 10x this value)
I don't know why the delta was not correctly calculated, but it is easy to see some improvement.

The files are on support folder, called `old-simple-slice.txt` and `new-slice-reuse.txt`.

I also tried to use a list.List from Golang core lib, but the situation got worse than using a slice. The result was:

```
name                                   old time/op    new time/op    delta
AccumulatorAppend/input_size_100-4       1.33ms ± 0%    1.88ms ± 0%   ~     (p=1.000 n=1+1)
AccumulatorAppend/input_size_1000-4      4.33ms ± 0%    4.79ms ± 0%   ~     (p=1.000 n=1+1)
AccumulatorAppend/input_size_74382-4      255ms ± 0%     314ms ± 0%   ~     (p=1.000 n=1+1)
AccumulatorAppend/input_size_382399-4     1.32s ± 0%     1.60s ± 0%   ~     (p=1.000 n=1+1)

name                                   old alloc/op   new alloc/op   delta
AccumulatorAppend/input_size_100-4       3.08MB ± 0%    3.03MB ± 0%   ~     (p=1.000 n=1+1)
AccumulatorAppend/input_size_1000-4      21.6MB ± 0%    21.5MB ± 0%   ~     (p=1.000 n=1+1)
AccumulatorAppend/input_size_74382-4     1.57GB ± 0%    1.57GB ± 0%   ~     (p=1.000 n=1+1)
AccumulatorAppend/input_size_382399-4    7.68GB ± 0%    7.68GB ± 0%   ~     (p=1.000 n=1+1)

name                                   old allocs/op  new allocs/op  delta
AccumulatorAppend/input_size_100-4        16.7k ± 0%     32.2k ± 0%   ~     (p=1.000 n=1+1)
AccumulatorAppend/input_size_1000-4       16.7k ± 0%     32.2k ± 0%   ~     (p=1.000 n=1+1)
AccumulatorAppend/input_size_74382-4      16.7k ± 0%     32.2k ± 0%   ~     (p=1.000 n=1+1)
AccumulatorAppend/input_size_382399-4     16.7k ± 0%     32.3k ± 0%   ~     (p=1.000 n=1+1)
```
