export function formatFinanceMoney(amountMinor: string, currency: string) {
  const formatter = new Intl.NumberFormat("ja-JP", { style: "currency", currency });
  const fractionDigits = formatter.resolvedOptions().maximumFractionDigits ?? 2;
  const minorAmount = BigInt(amountMinor);
  const isNegative = minorAmount < 0n;
  const absoluteMinorAmount = isNegative ? -minorAmount : minorAmount;
  const minorUnitScale = 10n ** BigInt(fractionDigits);
  const majorAmount = absoluteMinorAmount / minorUnitScale;
  const fractionAmount = absoluteMinorAmount % minorUnitScale;
  const integerParts = formatter
    .formatToParts(majorAmount)
    .filter((part) => part.type === "integer" || part.type === "group");
  const templateParts = formatter.formatToParts(isNegative ? -1n : 1n);

  // Intlの通貨記号・符号・区切り位置を保ち、実額だけはBigIntのまま組み立てる。
  return templateParts
    .flatMap((part) => {
      if (part.type === "integer") {
        return integerParts;
      }
      if (part.type === "fraction") {
        return [{ ...part, value: fractionAmount.toString().padStart(fractionDigits, "0") }];
      }
      return [part];
    })
    .map((part) => part.value)
    .join("");
}

export function formatFinanceDateTime(value: string) {
  return new Intl.DateTimeFormat("ja-JP", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}

export function formatAccountType(accountType: string) {
  const labels: Record<string, string> = {
    checking: "Checking",
    savings: "Savings",
    credit: "Credit",
    investment: "Investment",
    other: "Other",
  };
  return labels[accountType] ?? accountType;
}

export function formatAccountStatus(status: string) {
  return status === "active" ? "Active" : "Closed";
}

export function formatAccountMask(mask: string) {
  return `•••• ${mask.slice(-4)}`;
}
