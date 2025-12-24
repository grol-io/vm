#include <inttypes.h>
#include <stdint.h>
#include <stdio.h>

int main(void) {
  volatile int64_t counter = 1000000000;
  while (counter != 0) {
    counter--;
  }
  printf("Counter: %" PRId64 "\n", counter);
  return 0;
}
