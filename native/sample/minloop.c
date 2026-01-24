#include <inttypes.h>

int main(void) {
  int64_t counter = 1000000000;
  while (counter != 0) {
    counter--;
    asm volatile("" : "+r"(counter));
  }
  return (int)(counter);
}
