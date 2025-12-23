#include <stdint.h>
#include <stdio.h>

int main(void) {
    int64_t counter = 1000000000;
    while (counter !=0) {
        counter--;
    }
    printf("Counter: %lld\n", counter);
    return 0;
}
