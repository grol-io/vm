#include <inttypes.h>
#include <stdint.h>
#include <stdio.h>
#include <unistd.h>

#define MAX_ITOA 21
void itoa_println(int64_t num) {
  int sign = (num < 0) ? -1 : 1;
  char buf[MAX_ITOA];
  int i = MAX_ITOA - 1;
  buf[i--] = '\n';
  do {
    buf[i--] = '0' + sign * (num % 10);
    num /= 10;
  } while (num != 0);
  if (sign < 0) {
    buf[i--] = '-';
  }
  i++;
  write(1, buf + i, MAX_ITOA - i);
}

int main(void) {
  volatile int64_t counter = 1000000000;
  while (counter != 0) {
    counter--;
  }
  printf("Counter: %" PRId64 "\n", counter);
  return 0;
}
