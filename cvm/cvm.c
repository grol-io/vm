#include <inttypes.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>

typedef struct {
  int64_t data;
  uint8_t opcode;
} Operation;

typedef struct CPU {
  int64_t accumulator;
  int64_t pc;
  Operation *program;
  size_t program_size;
} CPU;

void run_program(CPU *cpu) {
  int64_t end = (int64_t)(cpu->program_size);
  while (cpu->pc < end) {
    Operation op = cpu->program[cpu->pc];
    switch (op.opcode) {
    case 0: // EXIT
      printf("Exit at PC %" PRId64 ": %" PRId64 "\n", cpu->pc,
             cpu->accumulator);
      exit(op.data);
    case 1: // LOAD
      cpu->accumulator = op.data;
      break;
    case 2: // ADD
      cpu->accumulator += op.data;
      break;
    case 3: // JNE
      if (cpu->accumulator != 0) {
        cpu->pc = op.data;
        continue;
      }
      break;
    default:
      fprintf(stderr, "Unknown opcode %d at PC %" PRId64 "\n", op.opcode,
              cpu->pc);
      break;
    }
    cpu->pc++;
  }
  printf("Program finished. Accumulator: %" PRId64 "\n", cpu->accumulator);
}

#define PACKED_SIZE 9

int main(int argc, char **argv) {
  if (argc < 2) {
    fprintf(stderr, "Usage: %s <program.vm>\n", argv[0]);
    return 1;
  }
  const char *filename = argv[1];
  FILE *f = fopen(filename, "rb");
  if (!f) {
    perror("Failed to open file");
    return 1;
  }
  CPU cpu = {0};
  fseek(f, 0, SEEK_END);
  cpu.program_size =
      ftell(f) / PACKED_SIZE; // packed size of Operation in file.
  fseek(f, 0, SEEK_SET);
  cpu.program = malloc(cpu.program_size * sizeof(Operation));
  if (!cpu.program) {
    perror("Failed to allocate memory for program");
    fclose(f);
    return 1;
  }
  for (size_t i = 0; i < cpu.program_size; i++) {
    if (fread(&cpu.program[i], PACKED_SIZE, 1, f) != 1) {
      perror("Failed to read operation");
      free(cpu.program);
      fclose(f);
      return 1;
    }
  }
  fclose(f);
  printf("Loaded program with %zu operations\n", cpu.program_size);
  run_program(&cpu);
  free(cpu.program);
  return 0;
}
