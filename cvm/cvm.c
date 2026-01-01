#include "cvm.h"
#include <inttypes.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

#ifndef DEBUG
#define DEBUG 0
#endif

#if DEBUG
#define DEBUG_PRINT(fmt, ...)                                                  \
  do {                                                                         \
    fprintf(stderr, fmt, __VA_ARGS__);                                         \
  } while (0)
#define DEBUG_ASSERT(expr)                                                     \
  do {                                                                         \
    if (!(expr)) {                                                             \
      fprintf(stderr, "Assertion failed: %s, file %s, line %d\n", #expr,       \
              __FILE__, __LINE__);                                             \
      exit(1);                                                                 \
    }                                                                          \
  } while (0)
#else
#define DEBUG_PRINT(fmt, ...)                                                  \
  do {                                                                         \
  } while (0)
#define DEBUG_ASSERT(expr)                                                     \
  do {                                                                         \
  } while (0)
#endif

typedef int64_t Operation;

uint8_t get_opcode(Operation op) { return (uint8_t)(op & 0xFF); }

int64_t get_operand(Operation op) { return (int64_t)(op >> 8); }

typedef struct CPU {
  int64_t accumulator;
  int64_t pc;
  Operation *program;
  size_t program_size;
} CPU;

enum { StackSize = 256 };

// sys_print writes bytes from memory starting at addr to stdout
// Returns the number of bytes written or -1 on error
// relies on the VM layout where the str8 payload is contiguous in memory
// following the first word that stores the length in its low byte.
int64_t sys_print(Operation *memory, int addr, int offset) {
  // All bytes are contiguous in memory (including the length byte)
  uint8_t *data = ((uint8_t *)&memory[addr])+offset;
  int length = *data++;
  if (length == 0) {
    return 0;
  }
  ssize_t n = write(STDOUT_FILENO, data, length);
  if (n != length) {
    fprintf(stderr,
            "Failed to write all bytes: expected %d, got %zd\n",
            length, n);
    return -1;
  }
  return length;
}

void run_program(CPU *cpu) {
  int64_t end = (int64_t)(cpu->program_size);
  Operation stack[StackSize];
  int stack_ptr = -1;
  while (cpu->pc < end) {
    Operation op = cpu->program[cpu->pc];
    uint8_t opcode = get_opcode(op);
    int64_t operand = get_operand(op);
    switch (opcode) {
    case LoadI:
      DEBUG_PRINT("LoadI %" PRId64 " at PC %" PRId64 "\n", operand, cpu->pc);
      cpu->accumulator = operand;
      break;
    case AddI:
      DEBUG_PRINT("AddI %" PRId64 " at PC %" PRId64 "\n", operand, cpu->pc);
      cpu->accumulator += operand;
      break;
    case SubI:
      DEBUG_PRINT("SubI %" PRId64 " at PC %" PRId64 "\n", operand, cpu->pc);
      cpu->accumulator -= operand;
      break;
    case MulI:
      DEBUG_PRINT("MulI %" PRId64 " at PC %" PRId64 "\n", operand, cpu->pc);
      cpu->accumulator *= operand;
      break;
    case DivI:
      DEBUG_PRINT("DivI %" PRId64 " at PC %" PRId64 "\n", operand, cpu->pc);
      cpu->accumulator /= operand;
      break;
    case ModI:
      DEBUG_PRINT("ModI %" PRId64 " at PC %" PRId64 "\n", operand, cpu->pc);
      cpu->accumulator %= operand;
      break;
    case ShiftI: {
      int64_t shift_val = operand;
      DEBUG_PRINT("ShiftI %" PRId64 " at PC %" PRId64 "\n", shift_val, cpu->pc);
      if (shift_val < 0) {
        uint64_t tmp = (uint64_t)cpu->accumulator;
        tmp >>= (uint64_t)(-shift_val);
        cpu->accumulator = (int64_t)tmp;
      } else {
        cpu->accumulator <<= shift_val;
      }
    } break;
    case AndI:
      DEBUG_PRINT("AndI %" PRId64 " at PC %" PRId64 "\n", operand, cpu->pc);
      cpu->accumulator &= operand;
      break;
    case JNZ:
      DEBUG_PRINT("JNZ %" PRId64 " at PC %" PRId64 "\n", operand, cpu->pc);
      if (cpu->accumulator != 0) {
        cpu->pc += operand;
        continue;
      }
      break;
    case JNEG:
      DEBUG_PRINT("JNEG %" PRId64 " at PC %" PRId64 "\n", operand, cpu->pc);
      if (cpu->accumulator < 0) {
        cpu->pc += operand;
        continue;
      }
      break;
    case JPOS:
      DEBUG_PRINT("JPOS %" PRId64 " at PC %" PRId64 "\n", operand, cpu->pc);
      if (cpu->accumulator >= 0) {
        cpu->pc += operand;
        continue;
      }
      break;
    case JumpR:
      DEBUG_PRINT("JumpR %" PRId64 " at PC %" PRId64 "\n", operand, cpu->pc);
      cpu->pc += operand;
      continue;
    case LoadR:
      DEBUG_PRINT("LoadR  at PC %" PRId64 ", offset: %" PRId64 "\n", cpu->pc,
                  operand);
      DEBUG_ASSERT(cpu->pc + operand >= 0 &&
                   (size_t)(cpu->pc + operand) < cpu->program_size);
      cpu->accumulator = (int64_t)cpu->program[cpu->pc + operand];
      DEBUG_PRINT("       loaded value: %" PRId64 "\n", cpu->accumulator);
      break;
    case AddR:
      DEBUG_PRINT("AddR   at PC %" PRId64 ", offset: %" PRId64 "\n", cpu->pc,
                  operand);
      DEBUG_ASSERT(cpu->pc + operand >= 0 &&
                   (size_t)(cpu->pc + operand) < cpu->program_size);
      cpu->accumulator += (int64_t)cpu->program[cpu->pc + operand];
      DEBUG_PRINT("       result: %" PRId64 "\n", cpu->accumulator);
      break;
    case SubR:
      DEBUG_PRINT("SubR   at PC %" PRId64 ", offset: %" PRId64 "\n", cpu->pc,
                  operand);
      DEBUG_ASSERT(cpu->pc + operand >= 0 &&
                   (size_t)(cpu->pc + operand) < cpu->program_size);
      cpu->accumulator -= (int64_t)cpu->program[cpu->pc + operand];
      DEBUG_PRINT("       result: %" PRId64 "\n", cpu->accumulator);
      break;
    case MulR:
      DEBUG_PRINT("MulR   at PC %" PRId64 ", offset: %" PRId64 "\n", cpu->pc,
                  operand);
      DEBUG_ASSERT(cpu->pc + operand >= 0 &&
                   (size_t)(cpu->pc + operand) < cpu->program_size);
      cpu->accumulator *= (int64_t)cpu->program[cpu->pc + operand];
      DEBUG_PRINT("       result: %" PRId64 "\n", cpu->accumulator);
      break;
    case DivR:
      DEBUG_PRINT("DivR   at PC %" PRId64 ", offset: %" PRId64 "\n", cpu->pc,
                  operand);
      DEBUG_ASSERT(cpu->pc + operand >= 0 &&
                   (size_t)(cpu->pc + operand) < cpu->program_size);
      cpu->accumulator /= (int64_t)cpu->program[cpu->pc + operand];
      DEBUG_PRINT("       result: %" PRId64 "\n", cpu->accumulator);
      break;
    case StoreR:
      DEBUG_PRINT("StoreR at PC %" PRId64 ", offset: %" PRId64
                  ", value: %" PRId64 "\n",
                  cpu->pc, operand, cpu->accumulator);
      DEBUG_ASSERT(cpu->pc + operand >= 0 &&
                   (size_t)(cpu->pc + operand) < cpu->program_size);
      cpu->program[cpu->pc + operand] = (Operation)cpu->accumulator;
      break;
    case IncrR: {
      int8_t incrval = operand & 0xFF;
      int64_t addr = operand >> 8;
      DEBUG_PRINT("IncrR  at PC %" PRId64 ", offset: %" PRId64
                  ", by value: %d\n",
                  cpu->pc, addr, incrval);
      DEBUG_ASSERT(cpu->pc + addr >= 0 &&
                   (size_t)(cpu->pc + addr) < cpu->program_size);
      cpu->accumulator = (int64_t)(cpu->program[cpu->pc + addr]) + incrval;
      cpu->program[cpu->pc + addr] = (Operation)cpu->accumulator;
    } break;
    case Sys:
    case SysS: {
      uint8_t syscallid = operand & 0xFF;
      int64_t syscallarg = operand >> 8;
      int is_stack = (opcode == SysS);
      switch (syscallid) {
      case Exit:
        DEBUG_PRINT("Exit Syscall (%d) at PC %" PRId64 ", accumulator: %" PRId64
                    ", argument: %" PRId64 "\n",
                    syscallid, cpu->pc, cpu->accumulator, syscallarg);
        exit(syscallarg);
      case Sleep:
        if (syscallarg < 0 || syscallarg > 1000) {
          fprintf(stderr,
                  "ERR: Sleep syscall argument out of range at PC %" PRId64
                  ": %" PRId64 "\n",
                  cpu->pc, syscallarg);
          exit(1);
        }
        fprintf(stderr,
                "Sleeping for %" PRId64 " milliseconds at PC %" PRId64 "\n",
                syscallarg, cpu->pc);
        usleep(syscallarg * 1000);
        break;
      case Write: {
        int64_t addr = is_stack ? (stack_ptr - (int)syscallarg)
                                : (cpu->pc + syscallarg);
        DEBUG_PRINT("Write syscall at PC %" PRId64 ", addr: %" PRId64
                    ", from %s\n",
                    cpu->pc, addr, is_stack ? "stack" : "program");
        cpu->accumulator = sys_print(is_stack ? stack : cpu->program,
                                     (int)addr, is_stack? cpu->accumulator : 0);
        if (cpu->accumulator == -1) {
          fprintf(stderr, "ERR: Write syscall failed at PC %" PRId64 "\n",
                  cpu->pc);
          exit(1);
        }
      } break;
      default:
        fprintf(stderr, "ERR: Unknown syscall %d at PC %" PRId64 "\n",
                syscallid, cpu->pc);
        exit(1);
      }
    } break;
    case Call:
      stack_ptr++;
      stack[stack_ptr] = (Operation)(cpu->pc + 1);
      DEBUG_PRINT("Call   at PC %" PRId64 ", jumping %+" PRId64 ", SP=%d\n",
                  cpu->pc, operand, stack_ptr);
      cpu->pc += operand;
      continue;
    case Ret: {
      int64_t extra = operand;
      if (extra > 0) {
        stack_ptr -= (int)extra;
      }
      DEBUG_PRINT("Return at PC %" PRId64 ", to %" PRId64 ", SP=%d\n",
                  cpu->pc, (int64_t)stack[stack_ptr], stack_ptr);
      cpu->pc = (int64_t)stack[stack_ptr];
      stack_ptr--;
      continue;
    }
    case Push: {
      int64_t count = operand;
      for (int64_t i = 0; i < count; i++) {
        stack_ptr++;
        stack[stack_ptr] = 0;
      }
      stack_ptr++;
      stack[stack_ptr] = (Operation)cpu->accumulator;
      DEBUG_PRINT("Push   at PC %" PRId64 ", value %" PRId64 ", SP=%d\n",
                  cpu->pc, cpu->accumulator, stack_ptr);
    } break;
    case Pop: {
      cpu->accumulator = (int64_t)stack[stack_ptr];
      stack_ptr--;
      int64_t extra = operand;
      if (extra > 0) {
        stack_ptr -= (int)extra;
      }
      DEBUG_PRINT("Pop    at PC %" PRId64 ", value %" PRId64 ", SP=%d\n",
                  cpu->pc, cpu->accumulator, stack_ptr);
    } break;
    case LoadS: {
      int offset = (int)operand;
      cpu->accumulator = (int64_t)stack[stack_ptr - offset];
      DEBUG_PRINT("LoadS  at PC %" PRId64 ", offset %d, value %" PRId64
                  ", SP=%d\n",
                  cpu->pc, offset, cpu->accumulator, stack_ptr);
    } break;
    case StoreS: {
      int offset = (int)operand;
      stack[stack_ptr - offset] = (Operation)cpu->accumulator;
      DEBUG_PRINT("StoreS at PC %" PRId64 ", offset %d, value %" PRId64
                  ", SP=%d\n",
                  cpu->pc, offset, cpu->accumulator, stack_ptr);
    } break;
    case AddS: {
      int offset = (int)operand;
      cpu->accumulator += (int64_t)stack[stack_ptr - offset];
      DEBUG_PRINT("AddS   at PC %" PRId64 ", offset %d, result %" PRId64
                  ", SP=%d\n",
                  cpu->pc, offset, cpu->accumulator, stack_ptr);
    } break;
    case SubS: {
      int offset = (int)operand;
      cpu->accumulator -= (int64_t)stack[stack_ptr - offset];
      DEBUG_PRINT("SubS   at PC %" PRId64 ", offset %d, result %" PRId64
                  ", SP=%d\n",
                  cpu->pc, offset, cpu->accumulator, stack_ptr);
    } break;
    case MulS: {
      int offset = (int)operand;
      cpu->accumulator *= (int64_t)stack[stack_ptr - offset];
      DEBUG_PRINT("MulS   at PC %" PRId64 ", offset %d, result %" PRId64
                  ", SP=%d\n",
                  cpu->pc, offset, cpu->accumulator, stack_ptr);
    } break;
    case DivS: {
      int offset = (int)operand;
      cpu->accumulator /= (int64_t)stack[stack_ptr - offset];
      DEBUG_PRINT("DivS   at PC %" PRId64 ", offset %d, result %" PRId64
                  ", SP=%d\n",
                  cpu->pc, offset, cpu->accumulator, stack_ptr);
    } break;
    case IncrS: {
      int64_t arg = operand;
      int offset = (int)(arg >> 8);
      int8_t value = (int8_t)(arg & 0xFF);
      DEBUG_PRINT("IncrS  at PC %" PRId64 ", offset %d, by %d, SP=%d\n",
                  cpu->pc, offset, value, stack_ptr);
      stack[stack_ptr - offset] += (Operation)value;
      DEBUG_PRINT("IncrS  new value %" PRId64 "\n",
                  (int64_t)stack[stack_ptr - offset]);
    } break;
    case IdivS: {
      int offset = (int)operand;
      int64_t current = (int64_t)stack[stack_ptr - offset];
      stack[stack_ptr - offset] = (Operation)(current / cpu->accumulator);
      cpu->accumulator = current % cpu->accumulator;
      DEBUG_PRINT("IdivS  at PC %" PRId64 ", offset %d, value %" PRId64
                  " -> %" PRId64 ", remainder %" PRId64 ", SP=%d\n",
                  cpu->pc, offset, current, stack[stack_ptr - offset],
                  cpu->accumulator, stack_ptr);
    } break;
    case StoreSB: {
      int64_t arg = operand;
      int base_offset = (int)(arg >> 8);           // highest stack offset in the span
      uint8_t bytes_stack_index = (uint8_t)(arg & 0xFF);
      int bytes_offset = (int)stack[stack_ptr - (int)bytes_stack_index];
      int word_offset = bytes_offset / 8;
      int stack_index = stack_ptr - base_offset + word_offset;
      uint64_t old_value = (uint64_t)stack[stack_index];
      int inner_offset_bits = (bytes_offset % 8) * 8;
      uint64_t mask = ((uint64_t)0xFF) << inner_offset_bits;
      uint64_t new_value = (old_value & ~mask) |
                           (((uint64_t)(cpu->accumulator & 0xFF))
                            << inner_offset_bits);
      stack[stack_index] = (Operation)new_value;
      DEBUG_PRINT(
          "StoreSB at PC %" PRId64
          ", baseOffset %d, bytesStackIndex %u, bytesOffset %d, oldValue %" PRIx64
          " -> newValue %" PRIx64 ", SP=%d\n",
          cpu->pc, base_offset, bytes_stack_index, bytes_offset, old_value,
          new_value, stack_ptr);
    } break;
    default:
      fprintf(stderr, "ERR: Unknown opcode %d at PC %" PRId64 "\n", opcode,
              cpu->pc);
      exit(1);
    }
    cpu->pc++;
  }
  fprintf(stderr, "Program finished. Accumulator: %" PRId64 "\n",
          cpu->accumulator);
}

#define HEADER "\x01GROL VM" // matches cpu.HEADER
#define INSTR_SIZE sizeof(Operation)

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
  cpu.program_size = (ftell(f) - (sizeof(HEADER) - 1)) /
                     INSTR_SIZE; // packed size of Operation in file - header.
  cpu.program = malloc(cpu.program_size * INSTR_SIZE);
  if (!cpu.program) {
    perror("Failed to allocate memory for program");
    fclose(f);
    return 1;
  }
  fseek(f, 0, SEEK_SET);
  char header[sizeof(HEADER)];
  header[sizeof(HEADER) - 1] = '\0';
  if (fread(header, sizeof(HEADER) - 1, 1, f) != 1) {
    perror("Failed to read header");
    fclose(f);
    free(cpu.program);
    return 1;
  }
  if (strncmp(header, HEADER, sizeof(HEADER) - 1) != 0) {
    fprintf(stderr, "Invalid header: %s\n", header);
    fclose(f);
    free(cpu.program);
    return 1;
  }
  if (fread(cpu.program, INSTR_SIZE, cpu.program_size, f) != cpu.program_size) {
    perror("Failed to read operation");
    fclose(f);
    free(cpu.program);
    return 1;
  }
  fclose(f);
  DEBUG_PRINT("Loaded program with %zu operations\n", cpu.program_size);
  run_program(&cpu);
  free(cpu.program);
  return 0;
}
