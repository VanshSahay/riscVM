// 256-bit unsigned integer as 8 × 32-bit limbs (little-endian).
// Limb 0 is the least significant 32 bits.

pub type U256 = [u32; 8];

pub fn zero() -> U256 {
    [0u32; 8]
}

pub fn is_zero(a: &U256) -> bool {
    a.iter().all(|&x| x == 0)
}

pub fn eq(a: &U256, b: &U256) -> bool {
    a.iter().zip(b.iter()).all(|(x, y)| x == y)
}

pub fn lt(a: &U256, b: &U256) -> bool {
    for i in (0..8).rev() {
        if a[i] < b[i] {
            return true;
        }
        if a[i] > b[i] {
            return false;
        }
    }
    false
}

pub fn gt(a: &U256, b: &U256) -> bool {
    lt(b, a)
}

pub fn gte(a: &U256, b: &U256) -> bool {
    !lt(a, b)
}

pub fn add(a: &U256, b: &U256) -> U256 {
    let mut res = zero();
    let mut carry = 0u64;
    for i in 0..8 {
        let sum = (a[i] as u64) + (b[i] as u64) + carry;
        res[i] = sum as u32;
        carry = sum >> 32;
    }
    res
}

pub fn sub(a: &U256, b: &U256) -> U256 {
    let mut res = zero();
    let mut borrow = 0i64;
    for i in 0..8 {
        let diff = (a[i] as i64) - (b[i] as i64) - borrow;
        res[i] = diff as u32;
        borrow = if diff < 0 { 1 } else { 0 };
    }
    res
}

pub fn mul(a: &U256, b: &U256) -> U256 {
    let full = mul_full(a, b);
    let mut res = zero();
    res.copy_from_slice(&full[0..8]);
    res
}

pub fn mulmod(a: &U256, b: &U256, m: &U256) -> U256 {
    if is_zero(m) {
        return zero();
    }
    let full = mul_full(a, b);
    let (_, rem) = div_rem_512_256(&full, m);
    rem
}

fn mul_full(a: &U256, b: &U256) -> [u32; 16] {
    let mut result = [0u32; 16];
    for i in 0..8 {
        let mut carry = 0u64;
        for j in 0..8 {
            let idx = i + j;
            let product = (a[i] as u64) * (b[j] as u64) + (result[idx] as u64) + carry;
            result[idx] = product as u32;
            carry = product >> 32;
        }
        let mut k = i + 8;
        while carry > 0 && k < 16 {
            let sum = (result[k] as u64) + carry;
            result[k] = sum as u32;
            carry = sum >> 32;
            k += 1;
        }
    }
    result
}

pub fn div(a: &U256, b: &U256) -> U256 {
    let (q, _) = div_rem(a, b);
    q
}

// mod (remainder of a / b)
pub fn rem(a: &U256, b: &U256) -> U256 {
    let (_, r) = div_rem(a, b);
    r
}

pub fn div_rem(a: &U256, b: &U256) -> (U256, U256) {
    if is_zero(b) {
        return (zero(), zero());
    }
    let mut quotient = zero();
    let mut remainder = zero();
    for i in (0..256).rev() {
        // remainder <<= 1
        let mut carry = 0u32;
        for limb in remainder.iter_mut() {
            let next = (*limb >> 31) & 1;
            *limb = (*limb << 1) | carry;
            carry = next;
        }
        // Set LSB = bit i of a
        let word = i / 32;
        let bit = i % 32;
        if (a[word] >> bit) & 1 == 1 {
            remainder[0] |= 1;
        }
        if gte(&remainder, b) {
            remainder = sub(&remainder, b);
            quotient[word] |= 1 << bit;
        }
    }
    (quotient, remainder)
}

fn div_rem_512_256(a: &[u32; 16], b: &U256) -> (U256, U256) {
    if is_zero(b) {
        return (zero(), zero());
    }
    let mut quotient = zero();
    let mut remainder = zero();
    for i in (0..512).rev() {
        // remainder <<= 1
        let mut carry = 0u32;
        for limb in remainder.iter_mut() {
            let next = (*limb >> 31) & 1;
            *limb = (*limb << 1) | carry;
            carry = next;
        }
        let word = i / 32;
        let bit = i % 32;
        if (a[word] >> bit) & 1 == 1 {
            remainder[0] |= 1;
        }
        if gte(&remainder, b) {
            remainder = sub(&remainder, b);
            if i < 256 {
                let qword = i / 32;
                let qbit = i % 32;
                quotient[qword] |= 1 << qbit;
            }
        }
    }
    (quotient, remainder)
}

pub fn and(a: &U256, b: &U256) -> U256 {
    let mut res = zero();
    for i in 0..8 {
        res[i] = a[i] & b[i];
    }
    res
}

pub fn or(a: &U256, b: &U256) -> U256 {
    let mut res = zero();
    for i in 0..8 {
        res[i] = a[i] | b[i];
    }
    res
}

pub fn xor(a: &U256, b: &U256) -> U256 {
    let mut res = zero();
    for i in 0..8 {
        res[i] = a[i] ^ b[i];
    }
    res
}

pub fn not(a: &U256) -> U256 {
    let mut res = zero();
    for i in 0..8 {
        res[i] = !a[i];
    }
    res
}

// Arithmetic right shift (SAR in EVM terms)
pub fn sar(a: &U256, shift: &U256) -> U256 {
    // shift amount modulo 256
    let sh = shift[0] as usize & 0xFF;
    if sh == 0 {
        return *a;
    }
    if sh >= 256 {
        // sign-extend: all bits = sign bit of a
        let sign = (a[7] >> 31) as u32;
        return [sign.wrapping_neg(); 8];
    }
    let sign = (a[7] >> 31) as u32;
    let mut res = zero();
    let limb_shift = sh / 32;
    let bit_shift = sh % 32;
    for i in 0..(8 - limb_shift) {
        let src = i + limb_shift;
        if src < 8 {
            res[i] = a[src] >> bit_shift;
            if bit_shift != 0 && src + 1 < 8 {
                res[i] |= a[src + 1] << (32 - bit_shift);
            }
        }
    }
    // Fill upper bits with sign
    let sign_word = if sign != 0 { 0xFFFFFFFFu32 } else { 0 };
    for i in (8 - limb_shift)..8 {
        res[i] = sign_word;
    }
    // Handle sign extension in the top occupied limb
    if limb_shift < 8 {
        let top_idx = 7 - limb_shift;
        if bit_shift != 0 {
            res[top_idx] |= sign_word << (32 - bit_shift);
        }
    }
    res
}

pub fn shl(a: &U256, shift: &U256) -> U256 {
    let sh = shift[0] as usize & 0xFF;
    if sh == 0 {
        return *a;
    }
    if sh >= 256 {
        return zero();
    }
    let mut res = zero();
    let limb_shift = sh / 32;
    let bit_shift = sh % 32;
    for i in limb_shift..8 {
        let src = i - limb_shift;
        res[i] = a[src] << bit_shift;
        if bit_shift != 0 && src > 0 {
            res[i] |= a[src - 1] >> (32 - bit_shift);
        }
    }
    res
}

pub fn shr(a: &U256, shift: &U256) -> U256 {
    let sh = shift[0] as usize & 0xFF;
    if sh == 0 {
        return *a;
    }
    if sh >= 256 {
        return zero();
    }
    let mut res = zero();
    let limb_shift = sh / 32;
    let bit_shift = sh % 32;
    for i in 0..(8 - limb_shift) {
        let src = i + limb_shift;
        if src < 8 {
            res[i] = a[src] >> bit_shift;
            if bit_shift != 0 && src + 1 < 8 {
                res[i] |= a[src + 1] << (32 - bit_shift);
            }
        }
    }
    res
}

pub fn byte(a: &U256, n: &U256) -> U256 {
    let idx = n[0] as usize;
    if idx >= 32 {
        return zero();
    }
    let word = idx / 4;
    let byte_idx = idx % 4;
    let b = ((a[7 - word] >> ((3 - byte_idx) * 8)) & 0xFF) as u32;
    [b, 0, 0, 0, 0, 0, 0, 0]
}

pub fn signextend(a: &U256, n: &U256) -> U256 {
    let byte_count = n[0] as usize;
    if byte_count >= 32 {
        return *a;
    }
    let bit_pos = byte_count * 8 + 7;
    let word = bit_pos / 32;
    let bit = bit_pos % 32;
    let sign = (a[word] >> bit) & 1;
    if sign == 0 {
        return *a;
    }
    let mut res = *a;
    for i in word..8 {
        if i == word {
            // Set bits from bit_pos+1 to 31 to 1
            let mask = !((1u32 << (bit + 1)).wrapping_sub(1));
            res[i] |= mask;
        } else {
            res[i] = 0xFFFFFFFF;
        }
    }
    res
}

// Build U256 from big-endian bytes (for PUSH)
pub fn from_bytes_be(bytes: &[u8]) -> U256 {
    let mut res = zero();
    for (i, &b) in bytes.iter().rev().enumerate() {
        if i >= 32 {
            break;
        }
        let word = i / 4;
        let byte_idx = i % 4;
        res[word] |= (b as u32) << (byte_idx * 8);
    }
    res
}

// Write U256 to big-endian bytes (for RETURN)
pub fn to_bytes_be(val: &U256, out: &mut [u8]) {
    let len = out.len().min(32);
    for i in 0..len {
        let word = 7 - i / 4;
        let byte_idx = 3 - (i % 4);
        out[i] = ((val[word] >> (byte_idx * 8)) & 0xFF) as u8;
    }
}
