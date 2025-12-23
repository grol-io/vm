#include <stdint.h>
#include <inttypes.h>
#include <stdio.h>

int main(void) {
    int64_t counter = 1000000000;
    while (counter != 0) {
        counter--;
        asm volatile ("");
    }
    printf("Counter: %" PRId64 "\n", counter);
    return 0;
}
