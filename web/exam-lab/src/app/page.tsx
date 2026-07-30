'use client'

import Image from "next/image";
import { Box, Button } from "@mui/material";
import styles from "./page.module.css";
import { useState } from "react";

export default function Home() {
  const [counter, setCounter] = useState(0);
  return (
    <Box>
      <Box>{counter}</Box>
      <Button onClick={() => setCounter((prev) => prev + 1)}>Add One</Button>
    </Box>
  );
}
